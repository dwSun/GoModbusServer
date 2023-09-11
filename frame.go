package mbserver

import (
	"encoding/binary"
	"encoding/hex"
	"strconv"
	"time"
)

// Framer is the interface that wraps Modbus frames.
type Framer interface {
	Bytes() []byte
	Copy() Framer
	GetData() []byte
	GetFunction() uint8
	SetException(exception *Exception)
	SetData(data []byte)
}

// GetException retunrns the Modbus exception or Success (indicating not exception).
func GetException(frame Framer) (exception Exception) {
	function := frame.GetFunction()
	if (function & 0x80) != 0 {
		exception = Exception(frame.GetData()[0])
	}
	return exception
}

func RegisterAddressAndNumber(frame Framer) (register int, numRegs int, endRegister int) {
	data := frame.GetData()
	register = int(binary.BigEndian.Uint16(data[0:2]))
	numRegs = int(binary.BigEndian.Uint16(data[2:4]))
	endRegister = register + numRegs
	return register, numRegs, endRegister
}

func RegisterAddressAndValue(frame Framer) (int, uint16) {
	data := frame.GetData()
	register := int(binary.BigEndian.Uint16(data[0:2]))
	value := binary.BigEndian.Uint16(data[2:4])
	return register, value
}

// SetDataWithRegisterAndNumber sets the RTUFrame Data byte field to hold a register and number of registers
func SetDataWithRegisterAndNumber(frame Framer, register uint16, number uint16) {
	data := make([]byte, 4)
	binary.BigEndian.PutUint16(data[0:2], register)
	binary.BigEndian.PutUint16(data[2:4], number)
	frame.SetData(data)
}

// SetDataWithRegisterAndNumberAndValues sets the TCPFrame Data byte field to hold a register and number of registers and values
func SetDataWithRegisterAndNumberAndValues(frame Framer, register uint16, number uint16, values []uint16) {
	data := make([]byte, 5+len(values)*2)
	binary.BigEndian.PutUint16(data[0:2], register)
	binary.BigEndian.PutUint16(data[2:4], number)
	data[4] = uint8(len(values) * 2)
	copy(data[5:], Uint16ToBytes(values))
	frame.SetData(data)
}

// SetDataWithRegisterAndNumberAndBytes sets the TCPFrame Data byte field to hold a register and number of registers and coil bytes
func SetDataWithRegisterAndNumberAndBytes(frame Framer, register uint16, number uint16, bytes []byte) {
	data := make([]byte, 5+len(bytes))
	binary.BigEndian.PutUint16(data[0:2], register)
	binary.BigEndian.PutUint16(data[2:4], number)
	data[4] = byte(len(bytes))
	copy(data[5:], bytes)
	frame.SetData(data)
}

func frameToString(frame Framer, time time.Time) string {

	msg := "\nTime : " + time.String()
	msg = msg + "\nBytes : "
	bytes_str := hex.EncodeToString(frame.Bytes())

	for i := 0; i < len(bytes_str); i += 2 {
		msg = msg + " " + bytes_str[i:i+2]
	}

	functi := frame.GetFunction()
	address := frame.GetAddress()
	msg = msg + "\nFunctionCode : " + strconv.Itoa(int(functi))
	msg = msg + "\nFunctionName : " + functionCodeToString(functi)
	msg = msg + "\nAddress : " + strconv.Itoa(int(address))

	if rtuFrame, ok := frame.(*RTUFrame); ok {
		msg = msg + "\nRTUFrame"
		msg = msg + "\nCRC : " + strconv.Itoa(int(rtuFrame.CRC))
	}

	if tcpFrame, ok := frame.(*TCPFrame); ok {
		msg = msg + "\nTCPFrame"
		msg = msg + "\nTransactionIdentifier : " + strconv.Itoa(int(tcpFrame.TransactionIdentifier))
		msg = msg + "\nProtocolIdentifier : " + strconv.Itoa(int(tcpFrame.ProtocolIdentifier))
		msg = msg + "\nLength : " + strconv.Itoa(int(tcpFrame.Length))
	}

	switch functi {
	case ReadDiscreteInput_fc, ReadHoldingRegisters_fc, ReadInputRegisters_fc:
		register, numRegs, _ := RegisterAddressAndNumber(frame)
		msg = msg + "\nStart-Register  " + strconv.Itoa(register)
		msg = msg + "\nNumber of Registers  " + strconv.Itoa(numRegs)

	}

	return msg
}
