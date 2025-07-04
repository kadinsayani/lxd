package ucred

import (
	"context"
	"errors"
	"net"

	"golang.org/x/sys/unix"

	"github.com/canonical/lxd/lxd/endpoints/listeners"
	"github.com/canonical/lxd/lxd/request"
)

// ErrNotUnixSocket is returned when the underlying connection isn't a unix socket.
var ErrNotUnixSocket = errors.New("Connection isn't a unix socket")

// GetCred returns the credentials from the remote end of a unix socket.
func GetCred(conn *net.UnixConn) (*unix.Ucred, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return nil, err
	}

	var ucred *unix.Ucred
	var ucredErr error
	err = rawConn.Control(func(fd uintptr) {
		ucred, ucredErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return nil, err
	}

	if ucredErr != nil {
		return nil, ucredErr
	}

	return ucred, nil
}

// GetConnFromContext extracts the connection from the request context on a HTTP listener.
func GetConnFromContext(ctx context.Context) net.Conn {
	reqInfo := request.GetContextInfo(ctx)
	if reqInfo == nil {
		return nil
	}

	return reqInfo.Conn
}

// GetCredFromContext extracts the unix credentials from the request context on a HTTP listener.
func GetCredFromContext(ctx context.Context) (*unix.Ucred, error) {
	conn := GetConnFromContext(ctx)
	unixConnPtr, ok := conn.(*net.UnixConn)
	if !ok {
		bufferedUnixConnPtr, ok := conn.(listeners.BufferedUnixConn)
		if !ok {
			return nil, ErrNotUnixSocket
		}

		unixConnPtr = bufferedUnixConnPtr.Unix()
	}

	return GetCred(unixConnPtr)
}
