// Package process implements HomeWaf's signed, authenticated local plugin runtime.
package process

import (
	"HomeWaf/internal/licensing"
	"HomeWaf/plugin/proto"
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var TrustedKeysJSON = "{}"

const protocolVersion = 2
const tokenHeader = "x-homewaf-plugin-token"

func TrustConfigured() bool {
	keys, err := licensing.ParseKeyring(TrustedKeysJSON)
	return err == nil && len(keys) > 0
}

type ClientConfig struct {
	Binary  string
	Timeout time.Duration
	Output  io.Writer
}
type Client struct {
	config  ClientConfig
	once    sync.Once
	err     error
	cmd     *exec.Cmd
	conn    atomic.Pointer[grpc.ClientConn]
	stdin   io.WriteCloser
	exited  atomic.Bool
	cleanup string
}
type RPCClient struct{ conn *grpc.ClientConn }
type launch struct {
	Protocol int    `json:"protocol"`
	Token    string `json:"token"`
}
type ready struct {
	Protocol int    `json:"protocol"`
	Address  string `json:"address"`
}

func NewClient(config *ClientConfig) *Client { return &Client{config: *config} }
func (c *Client) Exited() bool               { return c.exited.Load() }
func (c *Client) Kill() {
	if conn := c.conn.Load(); conn != nil {
		conn.Close()
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
}
func (c *Client) Client() (*RPCClient, error) {
	c.once.Do(func() { c.err = c.start() })
	if c.err != nil {
		return nil, c.err
	}
	return &RPCClient{c.conn.Load()}, nil
}
func (r *RPCClient) Dispense(kind string) (any, error) {
	if kind != "ip_filter" {
		return nil, errors.New("unsupported plugin service")
	}
	return proto.NewGRPCClient(proto.NewIPFilterPluginClient(r.conn)), nil
}

func (c *Client) stage() (string, error) {
	keys, err := licensing.ParseKeyring(TrustedKeysJSON)
	if err != nil || len(keys) == 0 {
		return "", errors.New("no trusted HomeWaf plugin signing keys configured")
	}
	stat, err := os.Lstat(c.config.Binary)
	if err != nil {
		return "", err
	}
	if !stat.Mode().IsRegular() || stat.Size() > 256*1024*1024 {
		return "", errors.New("plugin must be a regular executable smaller than 256 MiB")
	}
	manifest, err := os.Open(c.config.Binary + ".sig.json")
	if err != nil {
		return "", err
	}
	doc, err := io.ReadAll(io.LimitReader(manifest, licensing.MaxDocumentSize+1))
	manifest.Close()
	if err != nil {
		return "", err
	}
	source, err := os.Open(c.config.Binary)
	if err != nil {
		return "", err
	}
	defer source.Close()
	dir, err := os.MkdirTemp("", "homewaf-plugin-")
	if err != nil {
		return "", err
	}
	c.cleanup = dir
	target := filepath.Join(dir, filepath.Base(c.config.Binary))
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(source, 256*1024*1024+1))
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || n > 256*1024*1024 {
		os.RemoveAll(dir)
		return "", errors.New("failed to stage plugin")
	}
	if err = licensing.VerifyPlugin(doc, hex.EncodeToString(hash.Sum(nil)), keys); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return target, nil
}

func (c *Client) start() error {
	binary, err := c.stage()
	if err != nil {
		return err
	}
	started := false
	defer func() {
		if !started {
			c.Kill()
			os.RemoveAll(c.cleanup)
		}
	}()
	if c.config.Timeout <= 0 {
		c.config.Timeout = 30 * time.Second
	}
	if c.config.Output == nil {
		c.config.Output = io.Discard
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	c.cmd = exec.Command(binary)
	c.cmd.Stderr = c.config.Output
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = c.cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = c.cmd.Wait()
		c.exited.Store(true)
		if conn := c.conn.Load(); conn != nil {
			conn.Close()
		}
		os.RemoveAll(c.cleanup)
	}()
	if err = json.NewEncoder(c.stdin).Encode(launch{protocolVersion, token}); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(stdout, 4096)
	type result struct {
		ready ready
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			resultCh <- result{err: err}
			return
		}
		var handshake ready
		err = json.Unmarshal(line, &handshake)
		resultCh <- result{handshake, err}
	}()
	var handshake ready
	select {
	case result := <-resultCh:
		if result.err != nil {
			return result.err
		}
		handshake = result.ready
	case <-time.After(c.config.Timeout):
		return errors.New("plugin handshake timed out")
	}
	host, portText, err := net.SplitHostPort(handshake.Address)
	port, portErr := strconv.Atoi(portText)
	if err != nil || portErr != nil || port < 1 || port > 65535 || host != "127.0.0.1" || handshake.Protocol != protocolVersion {
		return errors.New("unsupported plugin handshake")
	}
	go func() { _, _ = io.Copy(c.config.Output, reader) }()
	connection, err := grpc.NewClient(handshake.Address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, request, response any, connection *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, tokenHeader, token)
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
			defer cancel()
		}
		return invoker(ctx, method, request, response, connection, options...)
	}))
	if err != nil {
		return err
	}
	c.conn.Store(connection)
	if c.Exited() {
		connection.Close()
		return errors.New("plugin exited during startup")
	}
	started = true
	return nil
}

// Serve must be the first stdout-producing action in a plugin executable.
// Diagnostics belong on stderr. The parent supplies an ephemeral credential on stdin.
func Serve(implementation proto.IPFilterPluginServer) error {
	reader := bufio.NewReaderSize(os.Stdin, 4096)
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return err
	}
	var request launch
	if err = json.Unmarshal(line, &request); err != nil {
		return err
	}
	token, err := base64.RawURLEncoding.DecodeString(request.Token)
	if err != nil || len(token) != 32 || request.Protocol != protocolVersion {
		return errors.New("invalid plugin launch credential")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	server := grpc.NewServer(grpc.MaxRecvMsgSize(4*1024*1024), grpc.UnaryInterceptor(func(ctx context.Context, requestBody any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		values, _ := metadata.FromIncomingContext(ctx)
		supplied := values.Get(tokenHeader)
		if len(supplied) != 1 || subtle.ConstantTimeCompare([]byte(supplied[0]), []byte(request.Token)) != 1 {
			return nil, status.Error(codes.Unauthenticated, "invalid plugin credential")
		}
		return handler(ctx, requestBody)
	}))
	proto.RegisterIPFilterPluginServer(server, implementation)
	if err = json.NewEncoder(os.Stdout).Encode(ready{protocolVersion, listener.Addr().String()}); err != nil {
		return fmt.Errorf("plugin readiness: %w", err)
	}
	go func() { _, _ = io.Copy(io.Discard, reader); server.Stop() }()
	return server.Serve(listener)
}
