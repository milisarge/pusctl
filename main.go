package main

import (
	"fmt"
	"encoding/json"
	"time"
	"log"
	"os/exec"
	"strings"
	"net"
	"github.com/digitalocean/go-qemu/qmp"
	//"strconv"
	"context"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"html/template"
    "io"
    "bufio"
    "sync"
)

type Template struct {
    templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
    return t.templates.ExecuteTemplate(w, name, data)
}

// define functions for template in use
var funcMap = template.FuncMap{
    "subTime": sub_time,
}

func sub_time(a, b time.Time) time.Duration {
    return b.Sub(a)
}

func format_time(a time.Time) string {
	return a.Format("2006-01-02 15:04:05")
}

var qemu_ports map[string]string
//var hbs map[string]time.Time

// concurency safe
var hbs = struct{
    sync.RWMutex
    m map[string]time.Time
}{m: make(map[string]time.Time)}

type StatusResult struct {
	ID     string `json:"id"`
	Return struct {
		Running    bool   `json:"running"`
		Singlestep bool   `json:"singlestep"`
		Status     string `json:"status"`
	} `json:"return"`
}

func get_status(ip string ,port string) string {
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s:%s",ip,port), 1*time.Second)
	if err != nil {
	  log.Println(err)
    } else {
      monitor.Connect()
	  defer monitor.Disconnect()
	  q_cmd := []byte(`{ "execute": "query-status" }`)
	  raw, _ := monitor.Run(q_cmd)
      var result StatusResult
	  json.Unmarshal(raw, &result)
	  return result.Return.Status
	}
	return ""
}

func snapshot(monitor *qmp.SocketMonitor,op string) {
	komut := fmt.Sprintf(`{"command-line":%svm state102"}`,op)
	fmt.Println(komut)
	q_cmd := []byte(fmt.Sprintf(`{"command-line":%svm state102"}`,op))
	raw, _ := monitor.Run(q_cmd)

	var result StatusResult
	json.Unmarshal(raw, &result)
	fmt.Println(result.Return.Status)
}

func save_snapshot(ip string ,port string, job byte) {
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s:%s",ip,port), 1*time.Second)
	if err != nil {
	  log.Println(err)
    } else {
      monitor.Connect()
	  defer monitor.Disconnect()
	  //komut := fmt.Sprintf(`{"execute": "snapshot-save","arguments": {"job-id": "save0%v","tag": "state102","vmstate": "blk","devices": ["blk"]}}`,job)
	  komut := fmt.Sprintf(`{"execute": "human-monitor-command","arguments": {"command-line": "savevm state102"}}`)
	  //komut := fmt.Sprintf(`{"human-monitor-command": "savevm state102"}`)
	  fmt.Println(komut, port)
	  q_cmd := []byte(komut)
	  raw, _ := monitor.Run(q_cmd)
	  fmt.Println("savevm:",string(raw))
    }
}

func load_snapshot(ip string ,port string) {
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s:%s",ip,port), 1*time.Second)
	if err != nil {
	  log.Println(err)
    } else {
      monitor.Connect()
	  defer monitor.Disconnect()
	  //komut := `{"execute": "snapshot-load","arguments": {"job-id": "load0","tag": "state102","vmstate": "blk","devices": ["blk"]}}`
	  komut := fmt.Sprintf(`{"execute": "human-monitor-command","arguments": {"command-line": "loadvm state102"}}`)
	  fmt.Println(komut, port)
	  q_cmd := []byte(komut)
	  raw, _ := monitor.Run(q_cmd)
	  fmt.Println("loadvm:",string(raw))
    }
}

func migrate_node(ip string ,port string, t_ip string, t_port string) {
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s:%s",ip,port), 1*time.Second)
	if err != nil {
	  log.Println(err)
    } else {
      monitor.Connect()
	  defer monitor.Disconnect()
	  //komut := `{"execute": "snapshot-load","arguments": {"job-id": "load0","tag": "state102","vmstate": "blk","devices": ["blk"]}}`
	  komut := fmt.Sprintf(`{"execute": "human-monitor-command","arguments": {"command-line": "migrate -d tcp:%s:%s"}}`,t_ip, t_port)
	  fmt.Println(komut, port)
	  q_cmd := []byte(komut)
	  raw, _ := monitor.Run(q_cmd)
	  fmt.Println("migrate:",string(raw))
    }
}

func reboot_machine(ip string ,port string) {
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s:%s",ip,port), 1*time.Second)
	if err != nil {
	  log.Println(err)
    } else {
      monitor.Connect()
	  defer monitor.Disconnect()
	  //komut := `{"execute": "snapshot-load","arguments": {"job-id": "load0","tag": "state102","vmstate": "blk","devices": ["blk"]}}`
	  komut := fmt.Sprintf(`{"execute": "human-monitor-command","arguments": {"command-line": "system_reset"}}`)
	  fmt.Println(komut, port)
	  q_cmd := []byte(komut)
	  raw, _ := monitor.Run(q_cmd)
	  fmt.Println("reboot:",string(raw))
    }
}

func get_events(monitor *qmp.SocketMonitor) {
	stream, _ := monitor.Events(context.Background())
	fmt.Println("--")
	for e := range stream {
		log.Printf("EVENT: %s", e.Event)
	}
}

func response(udpServer net.PacketConn, addr net.Addr, buf []byte) {
	//fmt.Println("resp:",buf)
	mid  := ""
	msg := ""
	// job id
	//num := 1
	for i,bi := range buf{
		//fmt.Printf("%v:%x ",i,bi)
		if fmt.Sprintf("%x",bi) == "a" {
		  idb := make([]byte, buf[i+1])
		  idb = buf[i+2 : (i + int(buf[i+1]) + 2 )]
		  mid = string(idb)
		  continue
		} 
		if fmt.Sprintf("%x",bi) == "2a" {
		  cntb := make([]byte, buf[i+1])
		  cntb = buf[i+2 : (i + int(buf[i+1]) + 2 )]
		  msg = string(cntb)
		  continue
		} 
	}
	ip := strings.Split(addr.String(),":")[0]
	portq := ""
	mach_st := "?"
	pint := time.Now()
	if qemu_ports[ip] != "" {
	  portq = qemu_ports[ip]
	  if msg == "heartbeat" {
		mach_st = get_status("192.168.122.1",portq)
		go save_snapshot("192.168.122.1",portq, byte(5))
	  }
	  if msg == "savevm" {
		if time.Now().Sub(pint) < (2*time.Second) {
		go save_snapshot("192.168.122.1", portq, buf[7])
		mach_st="saving"
		} else {mach_st="not saving"}
		
	  }
	}
	hbs.RLock()
	if hbs.m[ip] != time.Now() {
	  pint = hbs.m[ip]
	}
	hbs.RUnlock()
	//fmt.Printf(string(buf))
	//fmt.Println("--")
	time_val := time.Now()
	//fmt.Println(time_val)
	hbs.RLock()
	hbs.m[ip] = time_val
	hbs.RUnlock()
	fmt.Printf("addr:%s mid:%s msg:%s int:%s machine:%s \n",addr, mid, msg, time_val.Sub(pint), mach_st)
	/*
	responseStr := fmt.Sprintf("time received: %v. Your message: %v!", time, string(buf))

	udpServer.WriteTo([]byte(responseStr), addr)
	*/
}

func hearbeat_server() {
	udpServer, err := net.ListenPacket("udp4", ":8811")
	if err != nil {
		log.Fatal(err)
	}
	defer udpServer.Close()

	for {
		buf := make([]byte, 128)
		_, addr, err := udpServer.ReadFrom(buf)
		if err != nil {
			continue
		}
		go response(udpServer, addr, buf)
	}	
}

func PingWeb(c echo.Context) error {
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"ok": true,
  })
}

func HomePage(c echo.Context) error {
    //jout := x_json()
	return c.Render(http.StatusOK,"index.html",map[string]interface{}{
		"index_menu" : "active",
		//"data" : jout,
    })
}

func web_server() {
	t := &Template{
		//templates: template.Must(template.ParseGlob("templates/*.html")),
		templates: template.Must(template.New("").Funcs(funcMap).ParseGlob("./templates/*.html")),
	}
	
	e := echo.New()
	e.Static("/static", "./static")
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Renderer = t
	
	// routes
	e.GET("/", HomePage)
	e.GET("/api/ping", PingWeb)
	e.GET("/api/pus/hb", HbStatus)
	
	e.POST("/api/pus/crash/:node", SendCrash)
	e.POST("/api/pus/migrate/:node", MigrateNode)
	
	e.Logger.Fatal(e.Start(":8810"))
}

func HbStatus(c echo.Context) error {
  
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"nodes": hbs.m,
  })
}

func SendCrash(c echo.Context) error {
  node := c.Param("node")
  send_cmd(node,"5000")
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"op": "ok",
  })
}

func MigrateNode(c echo.Context) error {
  node := c.Param("node")
  port := qemu_ports[node]
  // discovery yapılıp alınacak
  // geçici qemu_target alındı
  t_ip   := "192.168.122.1" 
  t_port := "4510" 
  migrate_node("192.168.122.1", port ,t_ip, t_port)
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"op": "ok",
  })
}

func checkHb(ticker* time.Ticker) {
	for {
	  select {
		case t := <-ticker.C:
		  fmt.Println("Tick at", t)
		  for node := range hbs.m {
			hbs.RLock()
			df := t.Sub(hbs.m[node])
			hbs.RUnlock()
			if df > (60 * time.Second) {
			  fmt.Println("hb_checker", node, "rebooting", df)
			  go reboot_machine("192.168.122.1",qemu_ports[node])
			} else if df > (4 * time.Second) {
			  fmt.Println("hb_checker", node, "crash", df)
			  go load_snapshot("192.168.122.1",qemu_ports[node])
			} else {
			  fmt.Println("hb_checker", node, "healthy", df)
			  //go save_snapshot("192.168.122.1",qemu_ports[node], byte(t.Second()))
			}
		  } 
	  }
	}
}

func _send_cmd() map[string]interface{}{
	// \x08\xd9\x02\x12\x05crash\x1a\x04info\x00
	cmd := "test"
    out, _ := exec.Command("sh", "-c",cmd).Output()
    var jout map[string]interface{}
    json.Unmarshal([]byte(strings.Replace(string(out),"\n","",-1)), &jout)
    return jout
}

func send_cmd(ip string, port string) {
	p :=  make([]byte, 512)
    conn, err := net.Dial("tcp4", ip+":"+port)
    if err != nil {
        fmt.Printf("Some error %v", err)
        return
    }
    fmt.Fprintf(conn, "\x08\xd9\x02\x12\x05crash\x1a\x04info\x00")
    _, err = bufio.NewReader(conn).Read(p)
    if err == nil {
        fmt.Printf("%s\n", p)
    } else {
        fmt.Printf("Some error %v\n", err)
    }
    conn.Close()
}

func main(){
	
	qemu_ports = map[string]string {
	    "192.168.122.189":"4401",
		"192.168.122.190":"4402",
		"192.168.122.191":"4403",
		"192.168.122.192":"4404",
		"192.168.122.193":"4405",
		"192.168.122.194":"4406",
		"192.168.122.195":"4407",
		"192.168.122.196":"4408",
		"192.168.122.197":"4409",
		"192.168.122.204":"4410",
		"192.168.122.205":"4411",
		"192.168.122.206":"4412",
		"192.168.122.207":"4413",
		"192.168.122.208":"4414",
		"192.168.122.209":"4415",
		"192.168.122.210":"4416",
		"192.168.122.211":"4417",
		"192.168.122.212":"4418",
		"192.168.122.213":"4419",
		"192.168.122.220":"4420",
		"192.168.122.239":"4444",
	}
	hbs.m = make(map[string]time.Time, 10)
	
	ticker := time.NewTicker(time.Second * 10)
    go checkHb(ticker)
    
	go web_server()
	hearbeat_server()
	
	/*
	ip_port := os.Args[1]
	cmd := os.Args[2]
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s",ip_port), 2*time.Second)
	if err != nil {log.Println(err)}

	monitor.Connect()
	defer monitor.Disconnect()
	if cmd == "status" {
	  get_status(monitor)
	}
	if cmd == "events" {
	  get_events(monitor)
	}
	if cmd == "save" || cmd == "load" {
	  snapshot(monitor,cmd)
	}
	*/
}
