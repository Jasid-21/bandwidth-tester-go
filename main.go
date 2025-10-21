package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func removeExtraSpaces(baseStr string) string {
	var finalStr string = ""
	var i int = 0
	for i = 0; i < len(baseStr); i++ {
		char := string(baseStr[i])
		if len(finalStr) == 0 && char != " " {
			finalStr += char
			continue
		}

		if len(finalStr) != 0 && (char != " " || string(finalStr[len(finalStr)-1]) != " ") {
			finalStr += char
			continue
		}
	}

	return finalStr
}

func getKernelNetReport(interfaceName string) ([]uint64, error) {
	bytes, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		fmt.Printf("Error reading net report file: %s", err)
		return nil, err
	}

	chosenLine, err := func() (string, error) {
		lines := strings.Split(string(bytes), "\n")
		lines = lines[2:]
		for _, line := range lines {
			var rawInterface string = strings.Split(line, ":")[0]
			if strings.TrimSpace(rawInterface) == interfaceName {
				finalLine := removeExtraSpaces(line)
				return finalLine, nil
			}
		}
		return "", errors.New("Net interface not found")
	}()

	if err != nil {
		fmt.Print(err)
		return nil, err
	}

	strReceive := strings.Split(chosenLine, " ")[1]
	strTransmit := strings.Split(chosenLine, " ")[9]

	receive, err := strconv.ParseUint(strReceive, 10, 64)
	if err != nil {
		fmt.Printf("%v", err)
		return nil, err
	}

	transmit, err := strconv.ParseUint(strTransmit, 10, 64)
	if err != nil {
		fmt.Printf("%v", err)
		return nil, err
	}

	return []uint64{receive, transmit}, nil
}

func printBandWidth(report0 []uint64, report1 []uint64, elapsed time.Duration) []float64 {
	downloadBandwidth := (float64(report1[0]-report0[0]) * 8) / (elapsed.Seconds() * 1000000)
	uploadBandwidth := (float64(report1[1]-report0[1]) * 8) / (elapsed.Seconds() * 1000000)

	fmt.Printf("Download: %f\n", downloadBandwidth)
	fmt.Printf("Upload: %f\n", uploadBandwidth)

	return []float64{downloadBandwidth, uploadBandwidth}
}

func saveReport(file *os.File, measurements []float64) {
	download := strconv.FormatFloat(measurements[0], 'f', -1, 64)
	upload := strconv.FormatFloat(measurements[1], 'f', -1, 64)

	line := time.Now().Format(time.RFC3339) + " " + download + " " + upload + "\n"
	if _, err := file.WriteString(line); err != nil {
		fmt.Printf("%v", err)
		return
	}
}

func readReport() (plotter.XYs, plotter.XYs, error) {
	file, err := os.ReadFile("./report.txt")
	if err != nil {
		return nil, nil, err
	}

	var uploads, downloads plotter.XYs
	lines := strings.Split(string(file), "\n")
	for _, line := range lines[:len(lines)-1] {
		splitedLine := strings.Split(line, " ")
		timestamp, err := time.Parse(time.RFC3339, splitedLine[0])
		if err != nil {
			return nil, nil, err
		}
		download, err := strconv.ParseFloat(splitedLine[1], 64)
		if err != nil {
			return nil, nil, err
		}
		upload, err := strconv.ParseFloat(splitedLine[2], 64)
		if err != nil {
			return nil, nil, err
		}

		downloads = append(downloads, plotter.XY{X: float64(timestamp.Unix()), Y: download})
		uploads = append(uploads, plotter.XY{X: float64(timestamp.Unix()), Y: upload})
	}

	return downloads, uploads, nil
}

func plotReport(downloads, uploads plotter.XYs) error {
	p := plot.New()
	p.Title.Text = "Bandwidth measurement"
	p.X.Label.Text = "Time (s)"
	p.Y.Label.Text = "Speed (Gbps)"

	downloadsLine, err := plotter.NewLine(downloads)
	if err != nil {
		return err
	}

	uploadsLine, err := plotter.NewLine(uploads)
	if err != nil {
		return err
	}

	downloadsLine.Color = plotutil.Color(0)
	uploadsLine.Color = plotutil.Color(1)

	p.Add(downloadsLine, uploadsLine)
	p.Legend.Add("Downloads", downloadsLine)
	p.Legend.Add("Uploads", uploadsLine)

	return p.Save(10*vg.Inch, 4*vg.Inch, "report.png")
}

func askForUserInput(message string) string {
	fmt.Print(message)
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	return input.Text()
}

func main() {
	userInput := askForUserInput("Plot saved report (if you select 'No' the program will write a new report)? [y/N]")
	if strings.ToLower(userInput) == "y" {
		downloads, uploads, err := readReport()
		if err != nil {
			fmt.Printf("%v", err)
			return
		}

		plotReport(downloads, uploads)
		return
	}

	os.WriteFile("./report.txt", []byte{}, 0644)

	file, err := os.OpenFile("./report.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}
	defer file.Close()

	lastReport, err := getKernelNetReport("wlan0")
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	startTime := time.Now()
	for {
		time.Sleep(1 * time.Second)
		report, err := getKernelNetReport("wlan0")
		if err != nil {
			fmt.Printf("%v", err)
			return
		}

		elapsed := time.Since(startTime)
		measurement := printBandWidth(lastReport, report, elapsed)

		saveReport(file, measurement)
		lastReport = report
		startTime = time.Now()
	}
}
