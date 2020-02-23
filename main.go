package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

var db = accessDB()

func main() {
	http.HandleFunc("/", Server)
	http.ListenAndServe(":8080", nil)
}

//Server : main function delivering responses
func Server(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "GET":
		respondingGet(w, r, db)
	case "POST":
		var input Fountain = extractInput(w, r)
		respondingPost(w, r, input, db)
	case "PUT":
		var input Fountain = extractInput(w, r)
		if input.State != "" {
			respondingPutState(w, r, input, db)
		} else {
			respondingPutLocation(w, r, input, db)
		}
	case "DELETE":
		var input Fountain = extractInput(w, r)
		respondingDelete(w, r, input, db)
	}
}

type responseOne struct {
	Done     bool
	Fountain Fountain
}

type nearest struct {
	Fountains []Fountain
}

type search struct {
	Latitude  float64
	Longitude float64
	Radius    float64
}

func respondingGet(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	body, _ := ioutil.ReadAll(r.Body)
	textBytes := []byte(body)
	input := search{}
	err := json.Unmarshal(textBytes, &input)
	if err != nil {
		panic(err.Error())
	}
	Latitude, Longitude, Radius := input.Latitude, input.Longitude, input.Radius
	var got []Fountain = searchNearest(db, Latitude, Longitude, Radius)
	fountains := nearest{got}
	b, _ := json.Marshal(fountains)
	fmt.Fprintf(w, "%s", string(b))
}

func respondingDelete(w http.ResponseWriter, r *http.Request, input Fountain, db *sql.DB) {
	Err, fountain := deleteDB(db, input.ID)
	res := responseOne{Err, Fountain{fountain.ID, fountain.Latitude, fountain.Longitude, fountain.State}}
	b, _ := json.Marshal(res)
	fmt.Fprintf(w, "%s", string(b))
}

func respondingPutState(w http.ResponseWriter, r *http.Request, input Fountain, db *sql.DB) {
	Err := updateStateDB(db, input.ID, input.State)
	fountain := selectDB(db, input.ID)
	res := responseOne{Err, Fountain{fountain.ID, fountain.Latitude, fountain.Longitude, fountain.State}}
	b, _ := json.Marshal(res)
	fmt.Fprintf(w, "%s", string(b))
}

func respondingPutLocation(w http.ResponseWriter, r *http.Request, input Fountain, db *sql.DB) {
	Err := updateLocationDB(db, input.ID, input.Latitude, input.Longitude)
	fountain := selectDB(db, input.ID)
	res := responseOne{Err, Fountain{fountain.ID, fountain.Latitude, fountain.Longitude, fountain.State}}
	b, _ := json.Marshal(res)
	fmt.Fprintf(w, "%s", string(b))
}

func updateLocationDB(db *sql.DB, ID int, lat float64, long float64) bool {
	err := checkInputError(lat, long, "usable")
	if err {
		stmt, err := db.Prepare("UPDATE fountains SET location=POINT(?,?) WHERE id=?;")
		var lat string = fmt.Sprintf("%f", lat)
		var long string = fmt.Sprintf("%f", long)
		_, err = stmt.Exec(lat, long, strconv.Itoa(ID))
		if err != nil {
			panic(err.Error())
		}
	} else {
		fmt.Println("Bad Input.\n Latitude range: [-90,90]\n Longitude range: [-180,180]\n State: [usable,faulty].")
		return false
	}
	return true
}

func respondingPost(w http.ResponseWriter, r *http.Request, input Fountain, db *sql.DB) {
	Err, i := insertDB(db, input.Latitude, input.Longitude, input.State)
	res := responseOne{Err, Fountain{int(i), input.Latitude, input.Longitude, input.State}}
	b, _ := json.Marshal(res)
	fmt.Fprintf(w, "%s", string(b))
}

func insertDB(db *sql.DB, lat float64, long float64, state string) (bool, int64) {
	err := checkInputError(lat, long, state)
	var result int64
	if err {
		latitude := fmt.Sprintf("%f", lat)
		longitude := fmt.Sprintf("%f", long)
		stmt, err := db.Prepare("INSERT INTO fountains(location,state) VALUES(POINT(?,?),?);")
		res, err := stmt.Exec(latitude, longitude, state)
		result, _ = res.LastInsertId()
		if err != nil {
			panic(err.Error())
		}
	} else {
		fmt.Println("Bad Input.\n Latitude range: [-90,90]\n Longitude range: [-180,180]\n State: [usable,faulty].")
		return err, 0
	}
	return true, result
}

func selectDB(db *sql.DB, ID int) Fountain {
	fountains, err := db.Query("SELECT id, ST_X(location), ST_Y(location),state FROM fountains WHERE id=" + strconv.Itoa(ID) + ";")
	if err != nil {
		panic(err.Error())
	}
	var fountain Fountain
	fountains.Next()
	err = fountains.Scan(&fountain.ID, &fountain.Latitude, &fountain.Longitude, &fountain.State)
	if err != nil {
		panic(err.Error())
	}
	return fountain
}

func searchNearest(db *sql.DB, Latitude float64, Longitude float64, Radius float64) []Fountain {
	err := checkInputError(Latitude, Longitude, "usable")
	got := []Fountain{}
	if err {
		Latitude := fmt.Sprintf("%f", Latitude)
		Longitude := fmt.Sprintf("%f", Longitude)
		Radius := fmt.Sprintf("%f", Radius)
		res, err := db.Query("SELECT id, ST_X(location),ST_Y(location), state " +
			"FROM (" +
			"SELECT id, location, state, r, " +
			"units * DEGREES( ACOS( " +
			"COS(RADIANS(latpoint)) " +
			"* COS(RADIANS(ST_X(location))) " +
			"* COS(RADIANS(longpoint) - RADIANS(ST_Y(location))) " +
			"+ SIN(RADIANS(latpoint)) " +
			"* SIN(RADIANS(ST_X(location))))) AS distance " +
			"FROM fountains " +
			"JOIN ( " +
			"SELECT " + Latitude + "  AS latpoint,  " + Longitude + " AS longpoint," +
			" " + Radius + " AS r, 111.045 AS units " +
			") AS p ON (1=1) " +
			"WHERE MbrContains(ST_GeomFromText( " +
			"CONCAT('LINESTRING(', " +
			"latpoint-(r/units),' ', " +
			"longpoint-(r /(units* COS(RADIANS(latpoint)))), " +
			"',', " +
			"latpoint+(r/units) ,' ', " +
			"longpoint+(r /(units * COS(RADIANS(latpoint)))), " +
			"')')),  location) " +
			") AS d " +
			"WHERE d.distance <= d.r " +
			"ORDER BY d.distance ASC;")
		if err != nil {
			panic(err.Error())
		}
		for res.Next() {
			var fountain Fountain
			err = res.Scan(&fountain.ID, &fountain.Latitude, &fountain.Longitude, &fountain.State)
			if err != nil {
				panic(err.Error())
			}
			got = append(got, fountain)
		}
	} else {
		fmt.Println("Bad Input.\n Latitude range: [-90,90]\n Longitude range: [-180,180]\n State: [usable,faulty].")
	}
	return got
}

func updateStateDB(db *sql.DB, ID int, state string) bool {
	err := checkInputError(0, 0, state)
	if err {
		stmt, err := db.Prepare("UPDATE fountains SET state = ? WHERE id=?;")
		_, err = stmt.Exec(state, ID)
		if err != nil {
			panic(err.Error())
		}
	} else {
		fmt.Println("Bad Input.\n Latitude range: [-90,90]\n Longitude range: [-180,180]\n State: [usable,faulty].")
		return false
	}
	return true
}

func deleteDB(db *sql.DB, ID int) (bool, Fountain) {
	fountains, err := db.Query("SELECT id, ST_X(location), ST_Y(location),state FROM fountains WHERE id=" + strconv.Itoa(ID) + ";")
	_, err = db.Query("DELETE FROM fountains WHERE id=" + strconv.Itoa(ID) + ";")
	if err != nil {
		panic(err.Error())
	} else {
		var fountain Fountain
		fountains.Next()
		err = fountains.Scan(&fountain.ID, &fountain.Latitude, &fountain.Longitude, &fountain.State)
		if err != nil {
			return false, fountain
		}
		return true, fountain
	}
}

func extractInput(w http.ResponseWriter, r *http.Request) Fountain {
	body, _ := ioutil.ReadAll(r.Body)
	textBytes := []byte(body)
	fountain := Fountain{}
	err := json.Unmarshal(textBytes, &fountain)
	if err != nil {
		panic(err)
	}
	return fountain
}

func checkInputError(lat float64, long float64, state string) bool {
	if lat < -90 || lat > 90 {
		return false
	}
	if long > 180 || long < -180 {
		return false
	}
	if state != "usable" && state != "faulty" {
		return false
	}
	return true
}

func accessDB() *sql.DB {
	fmt.Print("Enter password(root): ")
	var password string
	fmt.Scanln(&password)
	db, err := sql.Open("mysql", "root:"+password+"@tcp(127.0.0.1:3306)/nasonDB")
	if err != nil {
		panic(err.Error())
	}
	return db
}

//Fountain : drinking fountain object
type Fountain struct {
	ID        int     `json:"ID,omitempty"`
	Latitude  float64 `json:"Latitude,omitempty"`
	Longitude float64 `json:"Longitude,omitempty"`
	State     string  `json:"State,omitempty"`
}
