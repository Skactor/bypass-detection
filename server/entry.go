package server

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Skactor/bypass-detection/config"
	"github.com/Skactor/bypass-detection/logger"
	"github.com/Skactor/bypass-detection/models"
	"github.com/panjf2000/ants/v2"
	"io"
	"net"
	"strings"
)

// SHA1 hashes using sha1 algorithm
func SHA1(text []byte) string {
	algorithm := sha1.New()
	algorithm.Write(text)
	return hex.EncodeToString(algorithm.Sum(nil))
}

// Read message from a net.Conn
func Read(conn net.Conn) ([]byte, error) {
	reader := bufio.NewReader(conn)
	var buffer bytes.Buffer
	for {
		ba, isPrefix, err := reader.ReadLine()
		if err != nil {
			// if the error is an End Of File this is still good
			if err == io.EOF {
				break
			}
			return nil, err
		}
		buffer.Write(ba)
		if !isPrefix {
			break
		}
	}
	return buffer.Bytes(), nil
}

// Write message to a net.Conn
// Return the number of bytes returned
func Write(conn net.Conn, encoded string) (int, error) {
	writer := bufio.NewWriter(conn)
	number, err := writer.WriteString(encoded)
	if err == nil {
		err = writer.Flush()
	}
	return number, err
}

func handleConn(i interface{}) {
	conn := i.(net.Conn)
	for {
		content, err := Read(conn)
		if err != nil {
			logger.Logger.Errorf("Listener: Read error: %s", err)
			break
		}
		obj := models.OutputRequest{}
		err = json.Unmarshal(content, &obj)
		if err != nil {
			logger.Logger.Errorf("failed to parse content: %v", content)
			continue
		}
		logger.Logger.Infof("Listener: Received content: %s", content)
		response := fmt.Sprintf("Encoded: %s\n", SHA1(content))
		logger.Logger.Infof("Listener: Response: %s", strings.TrimSpace(response))

		num, err := Write(conn, response)
		if err != nil {
			logger.Logger.Errorf("Listener: Write Error: %s", err)
			break
		}
		logger.Logger.Infof("Listener: Wrote %d byte(s) to %s", num, conn.RemoteAddr().String())
	}
}

func StartServer(cfg *config.ServerConfig) {
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		logger.Logger.Fatalf("Failed to bind %s, error: %s", cfg.Address, err.Error())
	}
	logger.Logger.Noticef("Listening for connections on %s", cfg.Address)
	var connections []net.Conn

	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
		listener.Close()
	}()

	pool, _ := ants.NewPoolWithFunc(1024, handleConn)

	for {
		conn, e := listener.Accept()
		if e != nil {
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				logger.Logger.Errorf("accept temp err: %v", ne)
				continue
			}
			logger.Logger.Errorf("accept err: %v", e)
			return
		}
		logger.Logger.Infof("accepted connection: %s", conn.RemoteAddr().String())
		pool.Invoke(conn)
		connections = append(connections, conn)
	}
}
