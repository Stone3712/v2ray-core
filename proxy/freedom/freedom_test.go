package freedom

import (
	"bytes"
	"fmt"
	"net"
	"testing"

	"golang.org/x/net/proxy"

	"github.com/v2ray/v2ray-core/app"
	"github.com/v2ray/v2ray-core/app/dispatcher"
	"github.com/v2ray/v2ray-core/common/alloc"
	v2io "github.com/v2ray/v2ray-core/common/io"
	v2net "github.com/v2ray/v2ray-core/common/net"
	v2nettesting "github.com/v2ray/v2ray-core/common/net/testing"
	v2proxy "github.com/v2ray/v2ray-core/proxy"
	_ "github.com/v2ray/v2ray-core/proxy/socks"
	proxytesting "github.com/v2ray/v2ray-core/proxy/testing"
	proxymocks "github.com/v2ray/v2ray-core/proxy/testing/mocks"
	"github.com/v2ray/v2ray-core/shell/point"
	v2testing "github.com/v2ray/v2ray-core/testing"
	"github.com/v2ray/v2ray-core/testing/assert"
	"github.com/v2ray/v2ray-core/testing/servers/tcp"
	"github.com/v2ray/v2ray-core/testing/servers/udp"
)

func TestUDPSend(t *testing.T) {
	v2testing.Current(t)

	data2Send := "Data to be sent to remote"

	udpServer := &udp.Server{
		Port: 0,
		MsgProcessor: func(data []byte) []byte {
			buffer := make([]byte, 0, 2048)
			buffer = append(buffer, []byte("Processed: ")...)
			buffer = append(buffer, data...)
			return buffer
		},
	}

	udpServerAddr, err := udpServer.Start()
	assert.Error(err).IsNil()

	connOutput := bytes.NewBuffer(make([]byte, 0, 1024))
	ich := &proxymocks.InboundConnectionHandler{
		ConnInput:  bytes.NewReader([]byte("Not Used")),
		ConnOutput: connOutput,
	}

	protocol, err := proxytesting.RegisterInboundConnectionHandlerCreator("mock_ich",
		func(space app.Space, config interface{}) (v2proxy.InboundHandler, error) {
			ich.PacketDispatcher = space.GetApp(dispatcher.APP_ID).(dispatcher.PacketDispatcher)
			return ich, nil
		})
	assert.Error(err).IsNil()

	pointPort := v2nettesting.PickPort()
	config := &point.Config{
		Port: pointPort,
		InboundConfig: &point.ConnectionConfig{
			Protocol: protocol,
			Settings: nil,
		},
		OutboundConfig: &point.ConnectionConfig{
			Protocol: "freedom",
			Settings: nil,
		},
	}

	point, err := point.NewPoint(config)
	assert.Error(err).IsNil()

	err = point.Start()
	assert.Error(err).IsNil()

	data2SendBuffer := alloc.NewBuffer().Clear()
	data2SendBuffer.Append([]byte(data2Send))
	ich.Communicate(v2net.NewPacket(udpServerAddr, data2SendBuffer, false))
	assert.Bytes(connOutput.Bytes()).Equals([]byte("Processed: Data to be sent to remote"))
}

func TestSocksTcpConnect(t *testing.T) {
	v2testing.Current(t)
	port := v2nettesting.PickPort()

	data2Send := "Data to be sent to remote"

	tcpServer := &tcp.Server{
		Port: port,
		MsgProcessor: func(data []byte) []byte {
			buffer := make([]byte, 0, 2048)
			buffer = append(buffer, []byte("Processed: ")...)
			buffer = append(buffer, data...)
			return buffer
		},
	}
	_, err := tcpServer.Start()
	assert.Error(err).IsNil()

	pointPort := v2nettesting.PickPort()
	config := &point.Config{
		Port: pointPort,
		InboundConfig: &point.ConnectionConfig{
			Protocol: "socks",
			Settings: []byte(`{"auth": "noauth"}`),
		},
		OutboundConfig: &point.ConnectionConfig{
			Protocol: "freedom",
			Settings: nil,
		},
	}

	point, err := point.NewPoint(config)
	assert.Error(err).IsNil()

	err = point.Start()
	assert.Error(err).IsNil()

	socks5Client, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", pointPort), nil, proxy.Direct)
	assert.Error(err).IsNil()

	targetServer := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := socks5Client.Dial("tcp", targetServer)
	assert.Error(err).IsNil()

	conn.Write([]byte(data2Send))
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.CloseWrite()
	}

	dataReturned, err := v2io.ReadFrom(conn, nil)
	assert.Error(err).IsNil()
	conn.Close()

	assert.Bytes(dataReturned.Value).Equals([]byte("Processed: Data to be sent to remote"))
}
