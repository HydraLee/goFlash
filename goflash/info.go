package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"strconv"
	"strings"
	"time"
)

//'host': args.host, 'port': args.port, 'user': args.user, 'passwd': args.password, 'charset': 'utf8'
type Dsn struct {
	user     string
	password string
	host     string
	port     uint16
	charset  string
}

func (d Dsn) String() string {
	//return "user:" + d.user + "\npassword:" + d.password + "\nhost:" + d.host + "\nport:" + d.port + "\ncharset:" + d.charset
	dsn := fmt.Sprintf("%s:%s@%s(%s:%d)/%s", d.user, d.password, "tcp", d.host, d.port, "mysql")
	return dsn
}

type Args struct {
	startFile      string
	startPos       uint32
	stopFile       string
	stopPos        uint32
	startTime      time.Time
	stopTime       time.Time
	noPK           bool
	flashBack      bool
	stopNever      bool
	backInterval   string
	onlyDml        bool
	sqlType        []string
	tables         []string
	databases      []string
	serverId       string
	eofFile        string
	eofPos         uint32
	binlogArray    []string
	outputFileName string
}

type master_status struct {
	masterFile        string
	masterPos         uint32
	Binlog_Do_DB      string
	Binlog_Ignore_DB  string
	Executed_Gtid_Set string
}

func NewDsn(user string, password string, host string, port uint16, charset string) (*Dsn, error) {
	if port == 0 {
		port = 3306
	}
	dsn := new(Dsn)
	dsn.user = user
	dsn.host = host
	dsn.port = port
	dsn.password = password
	dsn.charset = charset
	return dsn, nil

}

func NewArgs(dsn string, startFile string, startPos uint32, stopFile string, stopPos uint32, startTime string, stopTime string, noPK bool, flashBack bool, stopNever bool, backInterval string, onlyDml bool, sqlType string, tables string, databases string, outputFileName string) (*Args, error) {
	args := new(Args)
	if startFile == "" {
		return nil, errors.New("Lack of parameter: start_file")
	}
	if flashBack != false && stopNever != false {
		return nil, errors.New("Only one of flashback or stop-never can be True")
	}
	if noPK != false && stopNever != false {
		return nil, errors.New("Only one of flashback or no_pk can be True")
	}
	if (startTime != "" && (!isValidDatetime(startTime))) || (stopTime != "" && (!isValidDatetime(stopTime))) {
		return nil, errors.New("Incorrect datetime argument")
	}
	if startPos == 0 {
		startPos = 4
	}

	if stopFile == "" {
		args.stopFile = startFile
		stopFile = startFile
	} else {
		args.stopFile = stopFile
	}

	if startTime != "" {
		stime, err := time.Parse("2006-01-02 15:04:05", startTime)
		if err != nil {
			fmt.Println("start_time?????????????????????")
		}
		args.startTime = stime
	} else {
		stime, err := time.Parse("2006-01-02 15:04:05", "1980-01-01 00:00:00")
		if err != nil {
			return nil, errors.New("start_time?????????????????????")
		}
		args.startTime = stime

	}

	if stopTime != "" {
		stime, err := time.Parse("2006-01-02 15:04:05", stopTime)
		if err != nil {
			fmt.Println("start_time?????????????????????")
		}
		args.stopTime = stime
	} else {
		stime, err := time.Parse("2006-01-02 15:04:05", "2999-12-31 00:00:00")
		if err != nil {
			fmt.Println("start_time?????????????????????")
		}
		args.stopTime = stime

	}

	if sqlType != "" {
		args.sqlType = strings.Split(sqlType, ",")
	} else {
		fmt.Println("??????????????????????????????INSERT, UPDATE, DELETE")
	}

	if tables != "" {
		args.tables = strings.Split(tables, ",")
	} else {
		args.tables = nil
	}

	if outputFileName != "" {
		args.outputFileName = outputFileName
	} else {
		args.outputFileName = ""
	}

	if databases != "" {
		args.databases = strings.Split(databases, ",")
	} else {
		args.databases = nil
	}

	//???????????????
	fmt.Println("???????????????---------")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, errors.New("?????????????????????")
	}
	defer db.Close()
	//row := db.QueryRow("SHOW MASTER STATUS")
	row := db.QueryRow("show master status")

	ms := new(master_status)

	row.Scan(&ms.masterFile, &ms.masterPos, &ms.Binlog_Do_DB, &ms.Binlog_Ignore_DB, &ms.Executed_Gtid_Set)
	fmt.Print("???????????????binlog???????????????------")
	fmt.Println(ms.masterFile, ms.masterPos)
	args.eofFile = ms.masterFile
	args.eofPos = ms.masterPos

	//???????????????????????????binlog
	rows, err := db.Query("show master logs")
	defer rows.Close()
	if err != nil {
		return nil, errors.New("?????????????????????")
	}

	columns, _ := rows.Columns()
	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		//???sql.RawBytes???????????????????????????
		scanArgs[i] = &values[i]
	}
	//??????map??????????????????????????????
	var result []map[string]string
	for rows.Next() {
		res := make(map[string]string)
		rows.Scan(scanArgs...)
		for i, col := range values {
			res[columns[i]] = string(col)
		}
		result = append(result, res)
	}
	//??????????????????binlog?????????show master logs??????
	flag := false
	for _, r := range result {
		//????????????binlog???????????????binlog_array?????????
		if r["Log_name"] == startFile {
			flag = true
		}

		fileNum, err := strconv.Atoi(strings.Split(r["Log_name"], ".")[1])
		if err != nil {
			fmt.Println(err.Error())
		}
		startFileNum, err := strconv.Atoi(strings.Split(startFile, ".")[1])
		if err != nil {
			fmt.Println(err.Error())
		}
		stopFileNum, err := strconv.Atoi(strings.Split(stopFile, ".")[1])
		if err != nil {
			fmt.Println(err.Error())
		}
		//??????start file???stop file???????????????
		if startFileNum <= fileNum && fileNum <= stopFileNum {
			args.binlogArray = append(args.binlogArray, r["Log_name"])
		}

	}
	//??????start file stop file?????????????????????????????????
	if flag == false {
		return nil, errors.New("parameter error: start_file " + startFile + " not in mysql server")
	}
	fmt.Print("binlog????????????-----------------")
	fmt.Println(args.binlogArray)

	var mysqlServerId string
	row = db.QueryRow("SELECT @@server_id as server_id;")
	row.Scan(&mysqlServerId)
	args.serverId = mysqlServerId
	fmt.Println("??????serverid------------" + args.serverId)
	if args.serverId == "" {
		return nil, errors.New("?????????server id????????????")
	}

	args.startFile = startFile
	args.startPos = startPos
	args.stopPos = stopPos
	args.noPK = noPK
	args.flashBack = flashBack
	args.stopNever = stopNever
	args.backInterval = backInterval
	args.onlyDml = onlyDml

	return args, nil

}
