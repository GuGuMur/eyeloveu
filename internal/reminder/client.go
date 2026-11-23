package reminder

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const objectPath dbus.ObjectPath = "/local/eyeloveu/EyeLoveU/Daemon"

type Client struct {
	conn *dbus.Conn
}

func Connect() (*Client, error) {
	return connect(true)
}

func ConnectRunning() (*Client, error) {
	return connect(false)
}

func connect(start bool) (*Client, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	client := &Client{conn: conn}
	if _, err = client.Status(); err == nil {
		return client, nil
	}
	if !start {
		conn.Close()
		return nil, err
	}
	if err = launch("eyeloveu-daemon"); err == nil {
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
			if _, err = client.Status(); err == nil {
				return client, nil
			}
		}
	}
	conn.Close()
	return nil, fmt.Errorf("启动提醒后台: %w", err)
}

func (c *Client) Close() { c.conn.Close() }

func (c *Client) call(method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	return c.conn.Object(BusName, objectPath).CallWithContext(ctx, BusName+"."+method, 0, args...)
}

func (c *Client) Status() (Status, error) {
	var state Status
	var payload string
	err := c.call("Status").Store(&payload)
	if err == nil {
		err = json.Unmarshal([]byte(payload), &state)
	}
	return state, err
}

func (c *Client) Save(config Config) error {
	payload, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return c.call("Save", string(payload)).Err
}

func (c *Client) Act(action string) error         { return c.call("Act", action).Err }
func (c *Client) SetAutostart(enabled bool) error { return c.call("SetAutostart", enabled).Err }

func (c *Client) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// A process that is disconnecting cannot reliably send a method reply.
	err := c.conn.Object(BusName, objectPath).CallWithContext(ctx, BusName+".Quit", dbus.FlagNoReplyExpected).Err
	if err != nil {
		return err
	}
	for {
		var running bool
		err := c.conn.BusObject().CallWithContext(ctx, "org.freedesktop.DBus.NameHasOwner", 0, BusName).Store(&running)
		if err != nil || !running {
			return err
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func sibling(name string) (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), name), nil
}

func launch(name string, args ...string) error {
	path, err := sibling(name)
	if err != nil {
		return err
	}
	command := exec.Command(path, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	go command.Wait()
	return nil
}
