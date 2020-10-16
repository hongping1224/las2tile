// Copyright 2019 Hong-Ping Lo. All rights reserved.
// Use of this source code is governed by a BDS-style
// license that can be found in the LICENSE file.
package main

import (
	"testing"

	"github.com/hongping1224/lidario"
)

func TestOutput(t *testing.T) {
	files := findFile("./Output2", ".las")
	//files := findFile("./tmp2", "66_33_20191226SL - Scanner 1 - 191227_004220_VUX-1-LR - originalpoints_Tile by Point Number_5.las")
	for _, file := range files {
		t.Logf("%s", file)
		las, _ := lidario.NewLasFile(file, "r")
		if las.Header.MaxX-las.Header.MinX > 25 || las.Header.MaxY-las.Header.MinY > 25 {
			t.Errorf("file Is Larger than size : %s,  \n %v", file, las.Header)
			break
		}
		for i := 0; i < las.Header.NumberPoints; i++ {
			p, _ := las.LasPoint(i)
			d := p.PointData()
			if d.X > las.Header.MaxX || d.X < las.Header.MinX || d.Y > las.Header.MaxY || d.Y < las.Header.MinY {
				t.Errorf("Header %v", las.Header)
				t.Errorf("Point Out of range  %f , %f", d.X, d.Y)
				break
			}
		}

	}

}
