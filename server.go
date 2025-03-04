package mbserver

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/goburrow/serial"
)

// Server is a Modbus slave with allocated memory for discrete inputs, coils, etc.
type Server struct {
	// Debug enables more verbose messaging.
	Debug            bool
	slaveId          uint8
	listeners        []net.Listener
	ports            []serial.Port
	requestChan      chan *Request
	function         map[uint8]func(*Server, Framer) ([]byte, *Exception)
	DiscreteInputs   []byte
	Coils            []byte
	HoldingRegisters []byte
	InputRegisters   []byte
	last             []byte //buffer if there has been read more than one request buffer for new bytes so they do not get lost
	outChan          chan string
	closeChan        chan struct{} //channel to close all go-Routines if the server is no more used/closed
}

type Request struct {
	conn  io.ReadWriteCloser
	frame Framer
	t     time.Time //add time so we can log it as well
}

// could improve the constructor to make it clearer to use
func NewServer(id uint8) (*Server, error) {
	s := &Server{}

	if id == 0 {
		return nil, errors.New("modbus slave id must not be set to 0")
	}
	if id > 247 {
		return nil, errors.New("modbus slave id must not be greater than 247")
	}
	s.slaveId = id

	numberDiscreteInputs := 512
	numberCoils := 512
	numberHoldingRegisters := 512
	numberInputRegisers := 512
	// Allocate Modbus memory maps.
	s.DiscreteInputs = make([]byte, numberDiscreteInputs) //memory usage could be minimized
	s.Coils = make([]byte, numberCoils)
	s.HoldingRegisters = make([]byte, numberHoldingRegisters*2)
	s.InputRegisters = make([]byte, numberInputRegisers*2)

	s.function = make(map[uint8]func(*Server, Framer) ([]byte, *Exception))

	s.function[ReadCoils_fc] = ReadCoils
	s.function[ReadDiscreteInput_fc] = ReadDiscreteInputs
	s.function[ReadHoldingRegisters_fc] = ReadHoldingRegisters
	s.function[ReadInputRegisters_fc] = ReadInputRegisters
	s.function[WriteSingleCoil_fc] = WriteSingleCoil
	s.function[WriteHoldingRegister_fc] = WriteHoldingRegister
	s.function[WriteMultipleCoils_fc] = WriteMultipleCoils
	s.function[WriteHoldingRegisters_fc] = WriteHoldingRegisters

	s.requestChan = make(chan *Request)
	go s.handler()

	return s, nil
}

// RegisterFunctionHandler override the default behavior for a given Modbus function.
func (s *Server) RegisterFunctionHandler(funcCode uint8, function func(*Server, Framer) ([]byte, *Exception)) {
	s.function[funcCode] = function
}

func (s *Server) handle(request *Request) Framer {
	var exception *Exception
	var data []byte

	response := request.frame.Copy()

	function := request.frame.GetFunction()
	if _, ok := s.function[function]; ok {
		data, exception = s.function[function](s, request.frame)
		response.SetData(data)
	} else {
		exception = &IllegalFunction
	}

	if exception != &Success {
		response.SetException(exception)
	}

	return response
}

// All requests are handled synchronously to prevent modbus memory corruption.
func (s *Server) handler() {
	for {
		request, ok := <-s.requestChan
		if ok {
			if s.Debug {
				if s.outChan != nil { // already checked
					s.outChan <- "<<--Request<<--"
					s.outChan <- frameToString(request.frame.Copy(), request.t)
				}
			}
			response := s.handle(request)
			if s.Debug {
				if s.outChan != nil { // already checked
					s.outChan <- "-->>Response-->>"
					s.outChan <- frameToString(response.Copy(), time.Now())
				}
			}
			request.conn.Write(response.Bytes())
		} else {
			close(s.outChan)
			return
		}

	}
}

// Close stops listening to TCP/IP ports and closes serial ports.
func (s *Server) Close() {

	s.closeChan = make(chan struct{})
	close(s.closeChan)
	for _, listen := range s.listeners {
		listen.Close()
	}
	for _, port := range s.ports {
		port.Close()
	}
}

func (s *Server) ListenRequests() chan string {
	s.outChan = make(chan string, 1) //make channel asynchrone

	return s.outChan //need to return pointer or does it work this way
}
