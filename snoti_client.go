package gizwits_sac_go

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/pkg/errors"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	writeChannel    chan string = make(chan string, 500)
	errorLogChannel chan error  = make(chan error, 500)
	infoLogChannel  chan string = make(chan string, 500)
	warnLogChannel  chan string = make(chan string, 500)
)

func SetWriteChannelBuffer(channelBuffer int) {
	writeChannel = make(chan string, channelBuffer)
}

type SnotiClient struct {
	config     *Config
	tlsConn    *tls.Conn
	connReader *bufio.Reader
	isExited   bool
	isClosed   bool
	quitSignal chan os.Signal
}

// Should call Start method after new a SnotiClient
func (s *SnotiClient) Start() {
	s.watchAppExitSignal()

	// We set 3 times retry if no retry set
	retry := s.config.Retry
	if retry == 0 {
		retry = 3
	}

	for i := 0; i < retry; i++ {
		if !s.isExited {
			s.logErr(s.loop())
			time.Sleep(3 * time.Second)
		}
	}

	s.logInfo("[SnotiClient Info] ====> going to exit")
	time.Sleep(1 * time.Second)
}

/* SnotiClient need to catch the exit signal since it starts 4 background
long run goroutines to communicate to Gizwits Snoti API.
*/
func (s *SnotiClient) watchAppExitSignal() {
	go func() {
		signal.Notify(s.quitSignal, os.Interrupt, syscall.SIGTERM, syscall.SIGKILL)

		for qs := range s.quitSignal {
			s.logInfo("[SnotiClient Info] ====> catch signal: " + qs.String() + " and start to stop ...")
			s.isExited = true
		}
	}()
}

// Infinite loop methods wrapper
func (s *SnotiClient) loop() (err error) {
	tcpConn, err := net.DialTimeout("tcp", s.config.Addr, 3*time.Second)
	if err != nil {
		return errors.Wrap(err, "[SnotiClient Error] ====> Connect to Snoti API Server failed")
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	s.tlsConn = tls.Client(tcpConn, tlsConfig)
	s.connReader = bufio.NewReader(s.tlsConn)

	if err = s.login(); err != nil {
		return errors.Wrap(err, "[SnotiClient Error] ====> Login to Snoti API Server failed")
	}

	s.log()
	s.loopRemoteCtrl()
	s.heartbeat()
	s.loopWrite()
	err = s.loopRead()

	return
}

// Infinite loop to fetch remote control req from user callback
func (s *SnotiClient) loopRemoteCtrl() {
	if s.config.Callbacks.Write == nil {
		s.logInfo("[SnotiClient Info] ====> Quit Remote Control handler since no write callback defined")
		return
	}

	go func(sc *SnotiClient) {
		time.Sleep(2 * time.Second)

	REMOTE_CTRL_LOOP:
		for {
			if !sc.isStop() {
				writeData := sc.config.Callbacks.Write()
				if writeData == "" {
					time.Sleep(1 * time.Second)
				} else {
					data, err := sc.handleRemoteCtrlReq(writeData)
					if err == nil {
						writeChannel <- data
					} else {
						sc.logErr(errors.Wrap(err, "[SnotiClient Error] ====> Marshal Remote Control Req data failed"))
					}
				}
			} else {
				break REMOTE_CTRL_LOOP
			}
		}

		s.logInfo("[SnotiClient Info] ====> Quit Remote Control handler")
	}(s)
}

// Infinite loop to listen ticker which set in 4 minutes
func (s *SnotiClient) heartbeat() {
	go func(sc *SnotiClient) {
		timer := time.NewTicker(240 * time.Second) // 4 minutes

	HEARTBEAT_LOOP:
		for {
			if !sc.isClosed {
				select {
				case <-timer.C:
					writeChannel <- fmt.Sprintf("{\"cmd\": \"ping\"}")
				}
			} else {
				break HEARTBEAT_LOOP
			}
		}

		s.logInfo("[SnotiClient Info] ====> Quit Heartbeat handler")
	}(s)
}

// All the data which needed to send to Gizwits Snoti API will
// be put into a channel, and then in this method the loop listen
// the channel and write to the API server.
func (s *SnotiClient) loopWrite() {
	go func(sc *SnotiClient) {
		time.Sleep(2 * time.Second)

	WRITE_LOOP:
		for {
			if !sc.isClosed {
				select {
				case msg := <-writeChannel:
					err := sc.write(msg)
					if sc.isNetErr(err) {
						sc.logErr(err)
						break WRITE_LOOP
					}
				}
			} else {
				break WRITE_LOOP
			}
		}

		sc.logInfo("[SnotiClient Info] ====> Quit Write handler")
	}(s)
}

// Infinite loop to read the data from Gizwits Snoti API.
func (s *SnotiClient) loopRead() (err error) {
NOTI_READ_LOOP:
	for {
		if !s.isStop() {
			res, err := s.read()

			if !blankString(res) {
				deliveryId, ok := s.handleNotiData(res)
				if ok {
					err = s.eventAck(deliveryId)
				}
			}

			if s.isNetErr(err) {
				s.logErr(err)
				break NOTI_READ_LOOP
			}
		} else {
			break NOTI_READ_LOOP
		}
	}

	return
}

// Based on Gizwits Snoti API, client need to be authenticated.
func (s *SnotiClient) login() (err error) {
	loginReq := LoginReq{
		Cmd:           "login_req",
		PrefetchCount: s.config.PrefetchCount,
		Data:          s.config.AuthDataList,
	}

	bytes, err := json.Marshal(loginReq)
	if err != nil {
		return err
	}

	return s.write(string(bytes))
}

func (s *SnotiClient) read() (res string, err error) {
	deadline := time.Now().Add(time.Duration(s.config.Timeout.Read) * time.Second)
	err = s.tlsConn.SetReadDeadline(deadline)
	if err == nil {
		bytes, err := s.connReader.ReadBytes('\n')
		if err == nil {
			res = string(bytes)
		}
	}
	return res, err
}

func (s *SnotiClient) write(data string) (err error) {
	deadline := time.Now().Add(time.Duration(s.config.Timeout.Write) * time.Second)
	err = s.tlsConn.SetWriteDeadline(deadline)
	if err == nil {
		_, err = s.tlsConn.Write([]byte(data + "\n"))
	}
	return err
}

// Set closed if disconnect
func (s *SnotiClient) isNetErr(err error) bool {
	if err == io.EOF {
		s.isClosed = true
	}

	if operr, ok := err.(*net.OpError); ok && !operr.Temporary() && !operr.Timeout() {
		s.isClosed = true
	}

	return s.isClosed
}

// Connection closed or app exited means stop
func (s *SnotiClient) isStop() bool {
	return s.isClosed || s.isExited
}

func (s *SnotiClient) handleRemoteCtrlReq(t T) (reqString string, err error) {
	_, ok := t.(AttrsRCtrlReq)

	if !ok {
		_, ok = t.(RCtrlReq)
	}

	if ok {
		bytes, err := json.Marshal(t)

		if err == nil {
			reqString = string(bytes)
		}
	} else {
		err = errors.New("Invalid Remote Control Req type")
	}

	return
}

func (s *SnotiClient) handleNotiData(res string) (deliveryId int, ok bool) {
	jsonC, _ := gabs.ParseJSON([]byte(res))
	cmd, _ := jsonC.Path("cmd").Data().(string)

	switch cmd {
	case "event_push":
		fDeliveryId, ok := jsonC.Path("delivery_id").Data().(float64)
		if ok {
			deliveryId = int(fDeliveryId)
		}
	default:
		// TODO: nothing
	}

	// NOTE: Invoke user read callback
	// NOTE: It recommands that it is bad to run long time job in user callback
	s.config.Callbacks.Read(res)

	return
}

func (s *SnotiClient) eventAck(deliveryId int) error {
	return s.write(fmt.Sprintf(`{"cmd":"event_ack","delivery_id": %d }`, deliveryId))
}

// Listen all log channel in this method
func (s *SnotiClient) log() {
	go func(sc *SnotiClient) {
		logger := sc.config.Logger
		noLogger := logger == nil

	LOOP_LOGGER:
		for {
			if sc.isStop() {
				break LOOP_LOGGER
			} else {
				select {
				case err := <-errorLogChannel:
					if !noLogger {
						logger.Error(fmt.Sprintf("%+v", err))
					}
				case info := <-infoLogChannel:
					if !noLogger {
						logger.Info(info)
					}
				case warn := <-warnLogChannel:
					if !noLogger {
						logger.Warn(warn)
					}
				}
			}
		}
	}(s)
}

func (s *SnotiClient) logInfo(str string) {
	infoLogChannel <- str
}

func (s *SnotiClient) logErr(err error) {
	errorLogChannel <- err
}

func (s *SnotiClient) logWarn(str string) {
	warnLogChannel <- str
}

func blankString(input string) bool {
	return input == ""
}

func NewClient(config Config) *SnotiClient {
	return &SnotiClient{config: &config}
}
