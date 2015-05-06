package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"./summarizer"
)

// MacのCPU温度
func exec_cmd_mac_cpu_temperature(summ *summarizer.Summarizer) {
	re := regexp.MustCompile("\\d+(\\.\\d+)?")

	for {
		out, err := exec.Command("/Users/hnw/work/osx-cpu-temp/osx-cpu-temp").Output()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		v, err := strconv.ParseFloat(re.FindString(string(out)), 64)
		if err != nil {
			log.Println("cmd err")
		} else {
			summ.Send(v)
		}
		<-time.After(5 * time.Second)
	}
}

// Macのバッテリ残量
func exec_cmd_mac_battery_status(summ *summarizer.Summarizer) {
	re := regexp.MustCompile("-InternalBattery-0[ \t]+(\\d{1,3})+%")

	for {
		out, err := exec.Command("/usr/bin/pmset", "-g", "ps").Output()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		v, err := strconv.ParseFloat(re.FindStringSubmatch(string(out))[1], 64)
		if err != nil {
			log.Println("cmd err")
		} else {
			summ.Send(v)
		}
		<-time.After(5 * time.Second)
	}
}

func read_raspberrypi_temperature(summ *summarizer.Summarizer) {
	re := regexp.MustCompile("^(\\d+)\\s*$")
	temperature_file := "/sys/class/thermal/thermal_zone0/temp"
	for {
		file, err := os.Open(temperature_file)
		if err != nil {
			log.Fatal(err)
		}
		data := make([]byte, 100)
		count, err := file.Read(data)
		if err != nil {
			log.Fatal(err)
		}
		err = file.Close()
		if err != nil {
			log.Fatal(err)
		}
		v, err := strconv.ParseFloat(re.FindStringSubmatch(string(data[:count]))[1], 64)
		if err != nil {
			log.Println("cmd err")
			log.Println(err)
		} else {
			summ.Send(v / 1000.0)
		}
		<-time.After(5 * time.Second)
	}
}

func main() {
	summ_temp := summarizer.New()
	summ_temp.AddArchive(5*time.Second, 60)   // 5min
	summ_temp.AddArchive(15*time.Second, 120) // 30min
	summ_temp.AddArchive(5*time.Minute, 288)  // 24h
	summ_temp.AddArchive(1*time.Hour, 168)    // 1w
	summ_temp.AddArchive(24*time.Hour, 366)   // 1y

	//	go exec_cmd_mac_cpu_temperature(summ_temp)
	go read_raspberrypi_temperature(summ_temp)
	http.HandleFunc("/json/temperature/", summ_temp.Handler)
	/*
		summ_batt := summarizer.New()
		summ_batt.AddArchive(5*time.Second, 60)   // 5min
		summ_batt.AddArchive(15*time.Second, 120) // 30min
		summ_batt.AddArchive(5*time.Minute, 288)  // 24h
		summ_batt.AddArchive(1*time.Hour, 168)    // 1w
		summ_batt.AddArchive(24*time.Hour, 366)   // 1y

		go exec_cmd_mac_battery_status(summ_batt)
		http.HandleFunc("/json/battery/", summ_batt.Handler)
	*/
	http.Handle("/", http.FileServer(assetFS()))

	http.ListenAndServe(":18080", nil)
}
