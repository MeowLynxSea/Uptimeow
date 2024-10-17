package rcon

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/MeowLynxSea/Uptimeow/config"
	"github.com/robfig/cron/v3"
	"log"
	"net"
	"regexp"
	"strconv"
	"time"
)

type Connection struct {
	conn net.Conn
	pass string
	addr string
}

const (
	DataType_data_list = iota
	DataType_data_tps
	DataType_connection_error
	DataType_connection_success
	DataType_execution_error
)

var GlobalConfig config.ConfigData
var Cron = cron.New()
var isRunning bool

func InitRcon(callback func(data string)) {

	GlobalConfig = config.Load()

	var conn *Connection

	Cron.AddFunc("@every 5s", func() {
		command := [...]string{"list", "tps"}
		for _, v := range command {
			response, err := conn.SendCommand(v)
			if err != nil {
				// log.Println("[ERROR] Error executing command:", err)
				callback("{\"type\": " + strconv.Itoa(DataType_execution_error) + ", \"data\": \"Error executing command: " + err.Error() + "\"}")
				isRunning = false
				break
			} else {
				switch v {
				case "list":
					// 编译正则表达式
					onlinePlayerRegexp := regexp.MustCompile(`(\d+) of a max of (\d+) players online`)
					playerListRegexp := regexp.MustCompile(`online: ([^:]+)`)

					// 使用正则表达式提取信息
					onlinePlayerMatches := onlinePlayerRegexp.FindStringSubmatch(response)
					playerListMatches := playerListRegexp.FindStringSubmatch(response)

					// 检查是否匹配成功
					if onlinePlayerMatches != nil {
						// 提取online_player和max_player
						onlinePlayer, _ := strconv.Atoi(onlinePlayerMatches[1])
						maxPlayer, _ := strconv.Atoi(onlinePlayerMatches[2])

						var playerList string
						if playerListMatches != nil {
							playerList = playerListMatches[1]
						}

						// 创建一个结构体来保存数据
						type PlayerData struct {
							OnlinePlayer int      `json:"online_player"`
							MaxPlayer    int      `json:"max_player"`
							PlayerList   []string `json:"player_list"`
						}

						// 将玩家名字字符串分割成列表
						playerNames := regexp.MustCompile(`, `).Split(playerList, -1)

						// 实例化结构体并填充数据
						data := PlayerData{
							OnlinePlayer: onlinePlayer,
							MaxPlayer:    maxPlayer,
							PlayerList:   playerNames,
						}

						// 将结构体格式化为JSON
						jsonData, err := json.MarshalIndent(data, "", "  ")
						if err != nil {
							log.Println("[ERROR] Error marshalling JSON:", err)
						}

						callback("{\"type\": " + strconv.Itoa(DataType_data_list) + ", \"data\": " + string(jsonData) + "}")
					} else {
						log.Println("[ERROR] Could not extract the required information.")
						isRunning = false
						callback("{\"type\": " + strconv.Itoa(DataType_execution_error) + "}")
						break
					}
				case "tps":
					re := regexp.MustCompile(`§[a-zA-Z](\d+\.\d+|\d+)`)
					matches := re.FindAllStringSubmatch(response, -1)
					var numbers []string
					if len(matches) != 3 {
						isRunning = false
						callback("{\"type\": " + strconv.Itoa(DataType_execution_error) + "}")
						break
					}
					for _, match := range matches {
						if len(match) > 1 {
							numbers = append(numbers, match[1])
						}
					}
					callback(`{
					"type": ` + strconv.Itoa(DataType_data_tps) + `,
					"data": {
						"l1m": ` + numbers[0] + `,
						"l5m": ` + numbers[1] + `,
						"l15m": ` + numbers[2] + `
					}
				}`)
				}
			}
		}
	})

	for {
		isRunning = true
		log.Println("[INFO] Connecting to RCON server " + GlobalConfig.Rcon.Host + ":" + strconv.Itoa(GlobalConfig.Rcon.Port) + "...")

		var err error
		conn, err = NewConnection(GlobalConfig.Rcon.Host+":"+strconv.Itoa(GlobalConfig.Rcon.Port), GlobalConfig.Rcon.Password)
		if err != nil {
			callback("{\"type\": " + strconv.Itoa(DataType_connection_error) + ", \"data\": \"Error connecting to RCON server: " + err.Error() + "\"}")
			isRunning = false
		}

		if isRunning {
			callback("{\"type\": " + strconv.Itoa(DataType_connection_success) + "}")
			Cron.Start()
		}

		for isRunning {
			time.Sleep(10 * time.Nanosecond)
		}

		Cron.Stop()

		log.Println("[INFO] RCON server has disconnected. Trying to reconnect in 1 seconds...")
		time.Sleep(3 * time.Second)
	}
}

var uniqueID int32 = 0

func NewConnection(addr, pass string) (*Connection, error) {
	uniqueID++
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c := &Connection{conn: conn, pass: pass, addr: addr}
	if err := c.auth(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Connection) SendCommand(cmd string) (string, error) {
	err := c.sendCommand(2, []byte(cmd))
	if err != nil {
		return "", err
	}
	pkg, err := c.readPkg()
	if err != nil {
		return "", err
	}
	return string(pkg.Body), err
}

func (c *Connection) auth() error {
	c.sendCommand(3, []byte(c.pass))
	pkg, err := c.readPkg()
	if err != nil {
		return err
	}

	if pkg.Type != 2 || pkg.ID != uniqueID {
		return errors.New("incorrect password")
	}

	return nil
}

func (c *Connection) sendCommand(typ int32, body []byte) error {
	size := int32(4 + 4 + len(body) + 2)
	uniqueID += 1
	id := uniqueID

	wtr := binaryReadWriter{ByteOrder: binary.LittleEndian}
	wtr.Write(size)
	wtr.Write(id)
	wtr.Write(typ)
	wtr.Write(body)
	wtr.Write([]byte{0x0, 0x0})
	if wtr.err != nil {
		return wtr.err
	}

	c.conn.Write(wtr.buf.Bytes())
	return nil
}

func (c *Connection) readPkg() (pkg, error) {
	const bufSize = 4096
	b := make([]byte, bufSize)

	// Doesn't handle split messages correctly.
	read, err := c.conn.Read(b)
	if err != nil {
		return pkg{}, err
	}

	p := pkg{}
	rdr := binaryReadWriter{ByteOrder: binary.LittleEndian,
		buf: bytes.NewBuffer(b)}
	rdr.Read(&p.Size)
	rdr.Read(&p.ID)
	rdr.Read(&p.Type)
	body := [bufSize - 12]byte{}
	rdr.Read(&body)
	if rdr.err != nil {
		return p, rdr.err
	}
	p.Body = body[:read-14]
	return p, nil
}

type pkg struct {
	Size int32
	ID   int32
	Type int32
	Body []byte
}

type binaryReadWriter struct {
	ByteOrder binary.ByteOrder
	err       error
	buf       *bytes.Buffer
}

func (b *binaryReadWriter) Write(v interface{}) {
	if b.err != nil {
		return
	}
	if b.buf == nil {
		b.buf = new(bytes.Buffer)
	}
	b.err = binary.Write(b.buf, b.ByteOrder, v)
}

func (b *binaryReadWriter) Read(v interface{}) {
	if b.err != nil || b.buf == nil {
		return
	}
	b.err = binary.Read(b.buf, b.ByteOrder, v)
}
