package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/hongping1224/lidario"
)

func main() {
	workerCount := flag.Int("cpu", runtime.NumCPU(), "set Cpu usage")
	dir := flag.String("dir", "", "input Folder")
	overlap := flag.Float64("overlap", 5, "overlap between Patch (m)")
	size := flag.Float64("size", 25, "Patch Size(m)")
	out := flag.String("out", "./Output", "Output Folder")
	tmp := flag.String("tmp", "./tmp", "Temp Folder , use to save intermediate result")
	flag.Parse()
	runtime.GOMAXPROCS(*workerCount)

	inputRoot, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatal("Fail to read Abs input Folder path")
		return
	}

	fileinfo, err := os.Stat(inputRoot)
	if os.IsNotExist(err) {
		log.Fatal("path does not exist.")
	}

	lasPath := []string{inputRoot}
	if fileinfo.IsDir() {
		lasPath = findFile(inputRoot, ".las")
	}

	xmin, xmax, ymin, ymax, zmin, zmax := GetArea(lasPath)
	log.Println(xmin, xmax, ymin, ymax, zmin, zmax)
	xmin = ToNearestAnchorPoint(*overlap, *size, xmin)
	xmax = ToNearestAnchorPoint(*overlap, *size, xmax)
	ymin = ToNearestAnchorPoint(*overlap, *size, ymin)
	ymin = ToNearestAnchorPoint(*overlap, *size, ymin)
	zmin = ToNearestAnchorPoint(*overlap, *size, zmin)
	zmax = ToNearestAnchorPoint(*overlap, *size, zmax)

	shift := *size - *overlap

	radius := math.Sqrt((*size / 2.0 * *size / 2.0) * 2.0)

	log.Println("Start Tilling")

	for i, path := range lasPath {
		log.Printf("%d/%d", i+1, len(lasPath))
		start := time.Now()
		log.Println(path)
		las, err := lidario.NewLasFile(path, "r")
		if err != nil {
			log.Printf("%s read fail. Err : %v", path, err)
			continue
		}
		las.SetFixedRadiusSearchDistance(radius, false)
		done := make(chan bool)
		worker := 0
		for i := 0; (float64(i)*(shift))+xmin <= xmax; i++ {
			R := (float64(i) * (shift)) + xmin + *size
			L := (float64(i) * (shift)) + xmin
			centerx := (R + L) / 2
			if las.Header.MinX > R || las.Header.MaxX < L {
				continue
			}
			//log.Printf("centerx :%f", centerx)
			for j := 0; (float64(j)*(shift))+ymin <= ymax; j++ {
				U := (float64(j) * (shift)) + ymin + *size
				D := (float64(j) * (shift)) + ymin
				if las.Header.MinY > U || las.Header.MaxY < D {
					continue
				}
				centery := (U + D) / 2

				for worker == *workerCount {
					_, open := <-done
					worker--

					if open == false {
						break
					}
				}
				go func(las *lidario.LasFile, centerx, centery, R, L, U, D float64, i, j int) {
					ps := run(las, centerx, centery, R, L, U, D)
					if len(ps) != 0 {
						//Write Output
						_, err := os.Stat(*tmp)
						if os.IsNotExist(err) {
							os.MkdirAll(*tmp, os.ModePerm)
						}
						outpath := filepath.Join(*tmp, fmt.Sprintf("%d_%d_%s", i, j, filepath.Base(path)))
						outLas, err := lidario.InitializeUsingFile(outpath, las)
						if err != nil {
							log.Fatal(err)
							return
						}
						for _, point := range ps {
							outLas.AddLasPoint(point)
						}

						outLas.Close()
					}
					done <- true
				}(las, centerx, centery, R, L, U, D, i, j)
				worker++
			}
		}
		for {
			_ = <-done
			//log.Printf("%d Worker Left", worker)
			worker--
			if worker == 0 {
				break
			}
		}
		las.Close()
		elapsed := time.Now().Sub(start)
		log.Printf("%s:%v", path, elapsed)
		runtime.GC()
	}
	log.Println("Done Tilling, Start Merging")
	_, err = os.Stat(*out)
	if os.IsNotExist(err) {
		os.MkdirAll(*out, os.ModePerm)
	}
	files, err := filepath.Glob(filepath.Join(*tmp, "*"))
	if err != nil {
		log.Fatal(err)
	}
	sort.Strings(files)
	for len(files) > 0 {
		log.Printf("%d file left\n", len(files))
		target := filepath.Base(files[0])
		s := strings.Split(target, "_")
		index := 0
		com := make([]string, 0)
		for i, file := range files {
			a := strings.Split(filepath.Base(file), "_")
			if a[0] == s[0] && a[1] == s[1] {
				com = append(com, file)
			} else {
				index = i
				break
			}
		}
		maxsize := 100
		for i := 0; i < int(math.Ceil(float64(len(com))/float64(maxsize))); i++ {
			outpath := filepath.Join(*out, fmt.Sprintf("%s_%s_%d_Output.las", s[0], s[1], i))
			if len(com) < ((i + 1) * maxsize) {
				combine(com[(i*maxsize):len(com)], outpath)
			} else {
				combine(com[(i*maxsize):((i+1)*maxsize)], outpath)
			}
		}
		files = files[index:len(files)]
		if files[0] == com[0] {
			break
		}
	}

}

type data struct {
	Point lidario.LasPointer
	X, Y  float64
}

func run(las *lidario.LasFile, centerx, centery, R, L, U, D float64) []lidario.LasPointer {
	results := las.FixedRadiusSearch2D(centerx, centery)
	if results.Len() == 0 {
		return make([]lidario.LasPointer, 0)
	}
	points := make([]lidario.LasPointer, 0)
	for results.Len() > 0 {
		result, err := results.Pop()
		if err != nil {
			log.Println(err)
			continue
		}
		p, _ := las.LasPoint(result.Index)

		pdata := p.PointData()
		if pdata.X > R || pdata.X < L || pdata.Y > U || pdata.Y < D {
			continue
		}
		points = append(points, p)
	}
	return points

}

func combine(input []string, output string) {
	las, err := lidario.NewLasFile(input[0], "r")
	if err != nil {
		log.Fatalf("Fail to open las file %s , %v", input[0], err)
	}
	/*_, err = os.Stat(output)
	if os.IsNotExist(err) == false {
		return
	}*/

	outLas, err := lidario.InitializeUsingFile(output, las)
	if err != nil {
		log.Fatalf("Fail to init new las file %s , %v", output, err)
	}
	for i, s := range input {
		log.Printf("Combine %d/%d", i+1, len(input))
		if i != 0 {
			las, err = lidario.NewLasFile(s, "r")
			if err != nil {
				log.Fatalf("Fail to open las file %s , %v", s, err)
			}
		}
		if las.Header.MaxX-las.Header.MinX > 25 || las.Header.MaxY-las.Header.MinY > 25 {
			log.Println("SomethingWrong")
			log.Println(las.Header)
			log.Println(input[i])
		}
		for i := 0; i < las.Header.NumberPoints; i++ {
			p, _ := las.LasPoint(i)
			outLas.AddLasPoint(p)
		}
		las.Close()
		las = nil
		runtime.GC()
	}
	outLas.Close()
	outLas = nil
	runtime.GC()
}

//ToNearestAnchorPoint find nearest Anchor Point from value
func ToNearestAnchorPoint(overlap, size, value float64) float64 {
	anchor := value
	anchor = math.Floor(value/(size-overlap)) * (size - overlap)
	return anchor
}

//GetArea find the area cover by all las file
func GetArea(path []string) (xmin, xmax, ymin, ymax, zmin, zmax float64) {
	xmin = math.MaxFloat64
	ymin = math.MaxFloat64
	zmin = math.MaxFloat64
	xmax = -1 * math.MaxFloat64
	ymax = -1 * math.MaxFloat64
	zmax = -1 * math.MaxFloat64
	for _, las := range path {
		//log.Println(las)
		header, err := lidario.NewLasFile(las, "rh")
		if err != nil {
			log.Printf("%s read header fail. Err : %v", las, err)
		}
		xmin = math.Min(header.Header.MinX, xmin)
		ymin = math.Min(header.Header.MinY, ymin)
		zmin = math.Min(header.Header.MinZ, zmin)
		xmax = math.Max(header.Header.MaxX, xmax)
		ymax = math.Max(header.Header.MaxY, ymax)
		zmax = math.Max(header.Header.MaxZ, zmax)
	}
	return
}

func findFile(root string, match string) (file []string) {

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if strings.HasSuffix(info.Name(), match) {
			file = append(file, path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println("Total shp file : ", len(file))
	return file
}

func findFileWithPrefix(root string, match string) (file []string) {

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if strings.HasPrefix(info.Name(), match) {
			file = append(file, path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println("Total shp file : ", len(file))
	return file
}
