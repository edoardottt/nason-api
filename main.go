package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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
	var input Fountain = extractInput(w, r)
	w.Header().Set("Content-Type", "application/json")
	switch method {
	case "GET":
		fmt.Fprintf(w, "%s", method)
	case "POST":
		respondingPost(w, r, input, db)
	case "PUT":
		fmt.Fprintf(w, "%s", method)
	case "DELETE":
		fmt.Fprintf(w, "%s", method)
	}
}

type responseOne struct {
	Inserted bool
	Fountain Fountain
}

func respondingPutState(w http.ResponseWriter, r *http.Request, input Fountain, db *sql.DB) {
	Err := updateDB(db, input.ID, input.State)
	// row <- RETRIEVE SQL ROWS
	res := responseOne{Err, Fountain{int(i), row.Latitude, row.Longitude, input.State}}
	b, _ := json.Marshal(res)
	fmt.Fprintf(w, "%s", string(b))
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

func selectDB(db *sql.DB) {
	fountains, err := db.Query("SELECT id, ST_X(location), ST_Y(location),state FROM fountains")
	if err != nil {
		panic(err.Error())
	}
	for fountains.Next() {
		var fountain Fountain
		err := fountains.Scan(&fountain.ID, &fountain.Latitude, &fountain.Longitude, &fountain.State)
		if err != nil {
			panic(err.Error())
		}
		fmt.Println(fountain)
	}
}

func updateDB(db *sql.DB, id int, state string) bool {
	err := checkInputError(0, 0, state)
	if err {
		stmt, err := db.Prepare("UPDATE fountains SET state = ? WHERE id=?;")
		_, err = stmt.Exec(state, id)
		if err != nil {
			panic(err.Error())
		}
	} else {
		fmt.Println("Bad Input.\n Latitude range: [-90,90]\n Longitude range: [-180,180]\n State: [usable,faulty].")
		return false
	}
	return true
}

func deleteDB(db *sql.DB, id int) *sql.Rows {
	result, err := db.Query("SELECT FROM fountains WHERE id=" + string(id) + ";")
	_, err = db.Query("DELETE FROM fountains WHERE id=" + string(id) + ";")
	if err != nil {
		panic(err.Error())
	} else {
		fmt.Println("DELETED!")
		return result
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
