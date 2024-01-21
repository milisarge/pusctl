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
	"strconv"
	"context"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"html/template"
    "io"
    "bufio"
    "sync"
)

const host_ip = "192.168.122.1"
var latest = 1

type Machine struct {
  IP      string `json:"ip"`	
  HostIP  string `json:"host_ip"`	
  Port    string `json:"port"`
  TPort   string `json:"target_port"`
  Status  time.Time `json:"status"`	
  MStatus string `json:"m_status"`	
}

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

func deleteElement(slice []Machine, index int) []Machine {
   return append(slice[:index], slice[index+1:]...)
}

//var qemu_ports map[string]string
var qemu_ports = struct{
    sync.RWMutex
    m map[string]string
}{m: make(map[string]string)}

// concurency safe
var machines = struct{
    sync.RWMutex
    m map[string]Machine
}{m: make(map[string]Machine)} 
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

func define_machines() {
  for i, machine := range []Machine{
    Machine { IP: "192.168.122.189", HostIP: host_ip, Port: "4401", TPort: "4501", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.190", HostIP: host_ip, Port: "4402", TPort: "4502", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.191", HostIP: host_ip, Port: "4403", TPort: "4503", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.192", HostIP: host_ip, Port: "4404", TPort: "4504", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.193", HostIP: host_ip, Port: "4405", TPort: "4505", Status: time.Time{}, MStatus: "-" }, 
    /*
    Machine { IP: "192.168.122.194", HostIP: host_ip, Port: "4406", TPort: "4506", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.195", HostIP: host_ip, Port: "4407", TPort: "4507", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.196", HostIP: host_ip, Port: "4408", TPort: "4508", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.197", HostIP: host_ip, Port: "4409", TPort: "4509", Status: time.Time{}, MStatus: "-" }, 
    */
    //Machine { IP: "192.168.122.204", HostIP: host_ip, Port: "4410", TPort: "4510", Status: time.Time{}, MStatus: "-" }, 
    /*Machine { IP: "192.168.122.205", HostIP: host_ip, Port: "4411", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.206", HostIP: host_ip, Port: "4412", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.207", HostIP: host_ip, Port: "4413", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.208", HostIP: host_ip, Port: "4414", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.209", HostIP: host_ip, Port: "4415", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.210", HostIP: host_ip, Port: "4416", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.211", HostIP: host_ip, Port: "4417", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.212", HostIP: host_ip, Port: "4418", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.213", HostIP: host_ip, Port: "4419", Status: time.Time{}, MStatus: "-" }, 
    Machine { IP: "192.168.122.220", HostIP: host_ip, Port: "4420", Status: time.Time{}, MStatus: "-" },*/
    //Machine { IP: "192.168.122.239", HostIP: host_ip, Port: "4444", TPort: "4544", Status: time.Time{}, MStatus: "-" }, 
  } {
    machines.m[strconv.Itoa(i+1)] = machine
    latest = i+2
  }
}

// get ip related qemu comm port
func get_machine_by_ip(ip string) (string, Machine) {
  for k, m := range machines.m {
    if ip == m.IP {
	  return k,m
	}
  }
  return "0", Machine{}
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
	  //fmt.Println("*************************")
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

func shutdown_machine(ip string ,port string) {
	monitor, err := qmp.NewSocketMonitor("tcp", fmt.Sprintf("%s:%s",ip,port), 1*time.Second)
	if err != nil {
	  log.Println(err)
    } else {
      monitor.Connect()
	  defer monitor.Disconnect()
	  //komut := `{"execute": "snapshot-load","arguments": {"job-id": "load0","tag": "state102","vmstate": "blk","devices": ["blk"]}}`
	  komut := fmt.Sprintf(`{"execute": "quit"}`)
	  fmt.Println(komut, port)
	  q_cmd := []byte(komut)
	  raw, _ := monitor.Run(q_cmd)
	  fmt.Println("*************************")
	  fmt.Println("shutdown:",string(raw))
	  fmt.Println("*************************")
    }
}

func tmux_start_machine(id string) {
	//tmux new-session -d -s "pus${n}" "scripts/start_pus.sh $n"
	time.Sleep(3 * time.Second)
	fmt.Println("tmux start: ................................", id)
	cmd := "tmux new-session -d -s pus%s /opt/work/kernel/pusos/scripts/start_pus.sh %s -i"
    out, err := exec.Command("sh", "-c",fmt.Sprintf(cmd,id,id)).Output()
    fmt.Println("tmux out:",string(out))
    fmt.Println("tmux err:", err)
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
	for i,bi := range buf {
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
	machine_id, machine := get_machine_by_ip(ip)
	oper := "?"
	nowt := time.Now()

	if msg == "heartbeat" {
	  oper = get_status(machine.HostIP, machine.Port)
	  go save_snapshot(machine.HostIP, machine.Port, byte(5))
	}
	if msg == "savevm" {
	  // 2sn den küçükse hb e göre save snapshot alınacak	
	  if time.Now().Sub(nowt) < (2*time.Second) {
	    go save_snapshot(machine.HostIP, machine.Port, buf[7])
		oper="saving"
	  } else {oper="not saving"}
	}
	prev_time := machine.Status
	machine.Status = nowt
	machine.MStatus = get_status(machine.HostIP, machine.Port)
	machines.m[machine_id] = machine
	fmt.Printf("addr:%s mid:%s msg:%s int:%s machine:%s \n",addr, mid, msg, machine.Status.Sub(prev_time), oper)
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
	
	e.POST("/api/pus/crash/:node", SendCrash)
	e.POST("/api/pus/migrate/:node", MigrateNode)
	
	e.POST("/api/pus/machines/:ip/:port/:tport", RegisterMachine)
	e.GET("/api/pus/machines", GetMachines)
	
	e.Logger.Fatal(e.Start(":8810"))
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
  src_id, machine := get_machine_by_ip(node)
  // discovery yapılıp alınacak
  // geçici qemu_target alındı
  id := ""
  var target Machine
  for i, m := range machines.m {
    if m.MStatus == "inmigrate" {
	  target = m
	  id = i
	  break
	}
  }
  target.IP = machine.IP
  machines.m[id] = target
  // del source machine ip
  machine.IP = ""
  machines.m[src_id] = machine
  // target ip bulamazsa migrate yapılamaz
  if id == "" {
    return c.JSON(http.StatusOK, map[string]interface{}{
  	  "error": "migratable node not found",
    })
  }
  migrate_node(machine.HostIP, machine.Port ,target.HostIP, target.TPort)
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"found": "ok",
  	"machine" : id,
  })
}

func RegisterMachine(c echo.Context) error {
  ip := c.Param("ip")
  port := c.Param("port")
  tport := c.Param("tport")
  var m Machine
  m.HostIP = ip
  m.Port = port
  m.TPort = tport
  m.MStatus = "Reserved" //get_status(m.IP, m.Port)
  machines.m[strconv.Itoa(latest)] = m
  latest += 1
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"result": "ok",
	"machine" : m,
  })
}

func GetMachines(c echo.Context) error {
  return c.JSON(http.StatusOK, map[string]interface{}{
  	"machines": machines.m,
  })
}

func checkHb(ticker* time.Ticker) {
	for {
	  select {
		case t := <-ticker.C:
		  fmt.Println("Tick at", t)
		  for id, machine := range machines.m {
			df := t.Sub(machine.Status)
			fmt.Println(id, "mstatus:", machine.MStatus)
			// sadece çalışan makinelerin hb durumunu kontrol et
			if machine.MStatus == "running" {
				if df > (60 * time.Second) {
				  fmt.Println("hb_checker",id, machine.HostIP,  machine.IP, "rebooting", df)  
				  go reboot_machine(machine.HostIP,machine.Port)
				} else if df > (4 * time.Second) {
				  fmt.Println("hb_checker",id, machine.HostIP,  machine.IP, "crash", df)
				  go load_snapshot(machine.HostIP,machine.Port)
				} else {
				  fmt.Println("hb_checker",id, machine.HostIP,  machine.IP, "healthy", df)
				  //go save_snapshot(local_ip,qemu_ports[node], byte(t.Second()))
				}
			}
			//göçü tamamlamış makineyi sonlandır
			if machine.MStatus == "postmigrate" {
				fmt.Println("hb_checker",id, machine.HostIP,  machine.IP, "power_down", df)  
				go shutdown_machine(machine.HostIP,machine.Port)
				// todo: makineyi inmigrate modu ile tekrar başlat
				fmt.Println("starting......................................", id)
				go tmux_start_machine(id)
			}
			//update machine status
			machine.MStatus = get_status(machine.HostIP, machine.Port)
			machines.m[id] = machine
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
	
	define_machines()
	
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
