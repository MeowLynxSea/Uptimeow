package api

import (
	"database/sql"
	"encoding/json"
	"github.com/MeowLynxSea/Uptimeow/config"
	"github.com/MeowLynxSea/Uptimeow/internal/rcon"
	_ "github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/robfig/cron/v3"
	"github.com/wanghuiyt/ding"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var GlobalConfig config.ConfigData
var saveCron = cron.New()
var isOnline bool
var tps, tps5, tps15 float64
var onlinePlayer, maxPlayer int
var playerList []string
var db *sql.DB
var warnLevel int

const (
	warnLevelNormal   = 0
	warnLevelWarning  = 1
	warnLevelCritical = 2
)

type ServerData struct {
	Time         time.Time `json:"time"`
	IsOnline     bool      `json:"is_online"`
	Tps          float64   `json:"tps"`
	OnlinePlayer int       `json:"online_player"`
	MaxPlayer    int       `json:"max_player"`
}

// Response 是发送给WebSocket客户端的响应结构
type Response struct {
	Code int          `json:"code"`
	Data []ServerData `json:"data"`
}

type ServerInfo struct {
	ServerName        string `json:"server_name"`
	ServerAddress     string `json:"server_address"`
	ServerWebsite     string `json:"server_website"`
	ServerDescription string `json:"server_description"`
}

type DetailedInfo struct {
	Time         time.Time `json:"time"`
	IsOnline     bool      `json:"is_online"`
	Tps          float64   `json:"tps"`
	OnlinePlayer int       `json:"online_player"`
	MaxPlayer    int       `json:"max_player"`
	PlayerList   string    `json:"player_list,omitempty"`
}

func pushDingTalkBot(message string, msgtype string) {
	if GlobalConfig.Warn.DingTalkBot.Enabled {
		dingMsger := ding.Webhook{
			AccessToken: GlobalConfig.Warn.DingTalkBot.AccessToken,
			Secret:      GlobalConfig.Warn.DingTalkBot.Secret,
		}
		if GlobalConfig.Warn.DingTalkBot.AtMobile != "" {
			err := dingMsger.SendMessageText(message, GlobalConfig.Warn.DingTalkBot.AtMobile)
			if err != nil {
				log.Println("[ERROR] 钉钉机器人推送失败，原因: ", err)
			} else {
				log.Println("[INFO] 钉钉机器人推送[" + msgtype + "]成功")
			}
		} else {
			err := dingMsger.SendMessageText(message)
			if err != nil {
				log.Println("[ERROR] 钉钉机器人推送失败，原因: ", err)
			} else {
				log.Println("[INFO] 钉钉机器人推送[" + msgtype + "]成功")
			}
		}
	}
}

func init() {
	GlobalConfig = config.Load()
	warnLevel = 0

	pushDingTalkBot("【成功】Uptimeow 监控已上线", "成功消息")

	db, err := sql.Open("sqlite", "data/history.db")
	if err != nil {
		log.Fatal(err)
	}

	// 确保数据库连接是有效的
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// 创建表data
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS data (
		time_index DATETIME NOT NULL PRIMARY KEY,
		online BOOLEAN,
		tps INTEGER,
		online_player INTEGER,
		max_player INTEGER,
		player_list TEXT
	);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	saveCron.AddFunc("@every 10s", func() {
		currentTime := time.Now()
		// log.Println("[DEBUG] Saving data to database")
		err = db.Ping()
		if err != nil {
			log.Println(err)
		}
		if !isOnline {
			_, err = db.Exec("INSERT INTO data (time_index, online, tps, online_player, max_player, player_list) VALUES (?, ?, ?, ?, ?, ?)", currentTime.Format("2006-01-02 15:04:05"), 0, tps, 0, 0, "")
		} else {
			if tps != 0 {
				_, err = db.Exec("INSERT INTO data (time_index, online, tps, online_player, max_player, player_list) VALUES (?, ?, ?, ?, ?, ?)", currentTime.Format("2006-01-02 15:04:05"), isOnline, tps, onlinePlayer, maxPlayer, strings.Join(playerList, ","))
			}
		}
		if err != nil {
			log.Println("[ERROR] Failed to insert data into database:] ", err)
		}

		switch warnLevel {
		case warnLevelNormal:
			if !isOnline && GlobalConfig.Warn.EnabledType.Offline {
				warnLevel = warnLevelCritical
				pushDingTalkBot("【紧急】服务器离线\n经监测，服务器已离线，请尽快处理\n时间："+currentTime.Format("2006-01-02 15:04:05"), "异常告警")
				break
			}
			if tps < GlobalConfig.Warn.EnabledType.LowTps.Threold && GlobalConfig.Warn.EnabledType.LowTps.Enabled && tps != 0 {
				warnLevel = warnLevelWarning
				pushDingTalkBot("【警告】TPS过低报警\n服务器TPS低于设定值("+strconv.FormatFloat(GlobalConfig.Warn.EnabledType.LowTps.Threold, 'f', 2, 64)+")\n当前TPS："+strconv.FormatFloat(tps, 'f', 2, 64)+"\n时间："+currentTime.Format("2006-01-02 15:04:05"), "异常告警")
			}
		case warnLevelWarning:
			if !isOnline && GlobalConfig.Warn.EnabledType.Offline {
				warnLevel = warnLevelCritical
				pushDingTalkBot("【紧急】服务器离线\n经监测，服务器已离线，请尽快处理\n时间："+currentTime.Format("2006-01-02 15:04:05"), "异常告警")
				break
			}
			if tps >= GlobalConfig.Warn.EnabledType.LowTps.Threold && GlobalConfig.Warn.EnabledType.LowTps.Enabled {
				warnLevel = warnLevelNormal
				pushDingTalkBot("【恢复】服务器TPS恢复正常\n时间："+currentTime.Format("2006-01-02 15:04:05"), "成功消息")
			}
		case warnLevelCritical:
			if isOnline && GlobalConfig.Warn.EnabledType.Offline {
				warnLevel = warnLevelNormal
				pushDingTalkBot("【恢复】服务器已恢复在线\n时间："+currentTime.Format("2006-01-02 15:04:05"), "成功消息")
			}
		}
	})

	saveCron.Start()

	isOnline = false
	go rcon.InitRcon(callback)
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 允许所有跨域请求，或者你可以在这里添加更复杂的验证逻辑
		return true
	},
}

// WebSocketHandler 处理WebSocket连接
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	localDB, err := sql.Open("sqlite", "data/history.db")
	if err != nil {
		log.Fatal(err)
	}
	defer localDB.Close()

	// 确保数据库连接是有效的
	err = localDB.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// 将HTTP连接升级为WebSocket连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	// 在这里实现WebSocket的消息处理逻辑
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err)
			break
		}

		var resp Response
		clientTimeFormat := "2006/01/02 15:04:05"
		switch {
		case string(message)[:13] == "earlier than ":
			// 解析时间并获取数据
			// log.Println("Received earlier than request")
			reqTime, err := time.Parse(clientTimeFormat, strings.TrimPrefix(string(message), "earlier than "))
			if err != nil {
				log.Println("Error parsing time:", err)
				continue
			}
			resp.Data, err = getEarlierData(localDB, reqTime)
			if err != nil {
				log.Println("Error getting data from database:", err)
				continue
			}
		case string(message)[:11] == "later than ":
			// 解析时间并获取数据
			// log.Println("Received later than request")
			reqTime, err := time.Parse(clientTimeFormat, strings.TrimPrefix(string(message), "later than "))
			if err != nil {
				log.Println("Error parsing time:", err)
				continue
			}
			resp.Data, err = getLaterData(localDB, reqTime)
			if err != nil {
				log.Println("Error getting data from database:", err)
				continue
			}
		default:
			log.Println("Received unknown command " + string(message))
			continue
		}

		if err != nil {
			log.Println("Error getting data from database:", err)
			continue
		}

		resp.Code = 200
		// 序列化数据为JSON
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Println("Error marshaling response:", err)
			continue
		}

		// 发送数据给客户端
		if err := conn.WriteMessage(websocket.TextMessage, jsonResp); err != nil {
			log.Println("Error writing message:", err)
			break
		}
	}
}

func getLaterData(database *sql.DB, t time.Time) ([]ServerData, error) {
	dbTime := t.Format("2006-01-02 15:04:05")
	query := `SELECT time_index, online, tps, online_player, max_player
			  FROM data WHERE time_index > ? ORDER BY time_index ASC`
	rows, err := database.Query(query, dbTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []ServerData
	for rows.Next() {
		var sd ServerData
		if err := rows.Scan(&sd.Time, &sd.IsOnline, &sd.Tps, &sd.OnlinePlayer, &sd.MaxPlayer); err != nil {
			return nil, err
		}
		data = append(data, sd)
	}
	return data, nil
}

func getEarlierData(database *sql.DB, t time.Time) ([]ServerData, error) {
	dbTime := t.Format("2006-01-02 15:04:05")
	query := `SELECT time_index, online, tps, online_player, max_player
	          FROM data WHERE time_index < ? ORDER BY time_index DESC LIMIT 60`
	rows, err := database.Query(query, dbTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []ServerData
	for rows.Next() {
		var sd ServerData
		if err := rows.Scan(&sd.Time, &sd.IsOnline, &sd.Tps, &sd.OnlinePlayer, &sd.MaxPlayer); err != nil {
			return nil, err
		}
		data = append(data, sd)
	}
	reverse(data)
	return data, nil
}

func reverse(slice []ServerData) {
	last := len(slice) - 1
	for i := 0; i < len(slice)/2; i++ {
		slice[i], slice[last-i] = slice[last-i], slice[i]
	}
}

func APIHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应内容类型为JSON
	w.Header().Set("Content-Type", "application/json")

	// 解析请求参数
	queryParams := r.URL.Query()
	requestType := queryParams.Get("type")

	// 检查请求类型是否为 server_info
	switch requestType {
	case "server_info":
		// 准备要返回的数据
		serverInfo := ServerInfo{
			ServerName:        GlobalConfig.ServerInfo.Name,
			ServerAddress:     GlobalConfig.ServerInfo.Address,
			ServerWebsite:     GlobalConfig.ServerInfo.Website,
			ServerDescription: GlobalConfig.ServerInfo.Description,
		}

		// 创建响应结构
		response := struct {
			Code int        `json:"code"`
			Data ServerInfo `json:"data"`
		}{
			Code: 200,
			Data: serverInfo,
		}

		// 将响应结构序列化为JSON并写入响应体
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case "detailed_info":
		clientTimeFormat := "2006/01/02 15:04:05"
		t, err := time.Parse(clientTimeFormat, queryParams.Get("time"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		dbTime := t.Format("2006-01-02 15:04:05")

		localDB, err := sql.Open("sqlite", "data/history.db")
		if err != nil {
			log.Fatal(err)
		}
		defer localDB.Close()
		query := `SELECT time_index, online, tps, online_player, max_player, player_list
			  FROM data WHERE time_index > ? ORDER BY time_index ASC`
		rows, err := localDB.Query(query, dbTime)
		if err != nil {
			return
		}
		defer rows.Close()

		var data []DetailedInfo
		for rows.Next() {
			var sd DetailedInfo
			var playerList string
			if err := rows.Scan(&sd.Time, &sd.IsOnline, &sd.Tps, &sd.OnlinePlayer, &sd.MaxPlayer, &playerList); err != nil {
				return
			}
			sd.PlayerList = playerList
			data = append(data, sd)
		}
		// 创建响应结构
		response := struct {
			Code int          `json:"code"`
			Data DetailedInfo `json:"data"`
		}{
			Code: 200,
			Data: data[0],
		}

		// 将响应结构序列化为JSON并写入响应体
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Invalid request type", http.StatusBadRequest)
	}
}

func callback(data string) {
	// log.Println("[DEBUG] Receive callback data: " + data)

	var jsonData map[string]interface{}
	err := json.Unmarshal([]byte(data), &jsonData)
	if err != nil {
		log.Fatalln(err)
	}

	switch toInt(jsonData["type"]) {
	case rcon.DataType_connection_success:
		isOnline = true
		log.Println("[INFO] RCON connection success")
	case rcon.DataType_connection_error:
		isOnline = false
		tps, tps5, tps15, onlinePlayer, maxPlayer, playerList = 0, 0, 0, 0, 0, []string{}
		log.Println("[ERROR] RCON connection error")
	case rcon.DataType_execution_error:
		isOnline = false
		tps, tps5, tps15, onlinePlayer, maxPlayer, playerList = 0, 0, 0, 0, 0, []string{}
		log.Println("[ERROR] RCON execution error")
	case rcon.DataType_data_tps:
		log.Println("[DEBUG] TPS: " + strconv.FormatFloat(jsonData["data"].(map[string]interface{})["l1m"].(float64), 'f', -1, 64))
		tps = jsonData["data"].(map[string]interface{})["l1m"].(float64)
		tps5 = jsonData["data"].(map[string]interface{})["l5m"].(float64)
		tps15 = jsonData["data"].(map[string]interface{})["l15m"].(float64)
	case rcon.DataType_data_list:
		log.Println("[DEBUG] Player online: " + strconv.FormatFloat(jsonData["data"].(map[string]interface{})["online_player"].(float64), 'f', -1, 64) + "/" + strconv.FormatFloat(jsonData["data"].(map[string]interface{})["max_player"].(float64), 'f', -1, 64))
		onlinePlayer = int(jsonData["data"].(map[string]interface{})["online_player"].(float64))
		maxPlayer = int(jsonData["data"].(map[string]interface{})["max_player"].(float64))
		//read player list
		playerList = []string{}
		for _, player := range jsonData["data"].(map[string]interface{})["player_list"].([]interface{}) {
			playerList = append(playerList, player.(string))
		}
	}
}
