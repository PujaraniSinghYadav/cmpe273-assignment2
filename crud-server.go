package main
	
import (
	"fmt"
	"errors"
	"log"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"net/http"
	"github.com/gorilla/mux"
	"encoding/json"
	"strconv"
	"strings"
	"io/ioutil"
)	
	
type Location struct{
	Id int
	Name string
	Address string
	City string
	State string
	Zip string
	lat string
	lng string
}	

func JSON(loc Location) interface{} {
	str := map[string]interface{}{
		"id": loc.Id,
		"name": loc.Name,
		"address": loc.Address,
		"city": loc.City,
		"state": loc.State,
		"zip": loc.Zip,
		"coordinate": map[string]interface{}{
			"lat": loc.lat,
			"lng": loc.lng,
		},
	}

	return str
} 

//
// Fetch from Google Maps
//
func Fix(data string) string {
	return strings.Replace(data, " ", "+", -1)
}

func GetMapsLocation(loc Location) (float64,float64,error) {
	address := Fix(loc.Address) + ",+" + Fix(loc.City) + ",+" + Fix(loc.State)
	url := "http://maps.google.com/maps/api/geocode/json?address=" + address + "&sensor=false"
	
	res,err := http.Get(url)
	if err != nil {
		return 0,0,err
	}

	content,err := ioutil.ReadAll(res.Body)

	var c map[string]interface{}
	err = json.Unmarshal(content, &c)

	results := c["results"].([]interface{})
	rr := results[0].(map[string]interface{})
	geometry := rr["geometry"].(map[string]interface{})
	location := geometry["location"].(map[string]interface{})
	lat := location["lat"].(float64)
	lng := location["lng"].(float64)

	return lat,lng,err
}

//
// MongoDB operations
//
func ResetCollection(cname string) error {
	session,err := mgo.Dial("mongodb://foo:bar@ds035623.mongolab.com:35623/cmpe273")
	if err != nil {
		return err
	}

	defer session.Close()
	session.DB("cmpe273").C(cname).DropCollection()
	return nil
}

func OpenCollection(cname string) (*mgo.Collection,*mgo.Session,error) {
	session,err := mgo.Dial("mongodb://foo:bar@ds035623.mongolab.com:35623/cmpe273")
	if err != nil {
		return nil,nil,err
	}

 	session.SetMode(mgo.Monotonic, false)
	c := session.DB("cmpe273").C(cname)
	return c,session,nil
}

func InsertLocation(c *mgo.Collection, location Location) error {
	_,err := GetLocation(c, location.Id)
	if err == nil {
		fmt.Println("Location already exists.", location)
		return errors.New("already exists")
	}

	fmt.Println("InsertLocation. ", location)
	err = c.Insert(&location)
	return err
}	

func UpdateLocation(c *mgo.Collection, location Location) error {
	val,err := GetLocation(c, location.Id)
	if err != nil {
		fmt.Println("Location not found.", location)
		return errors.New("Not found")
	}	
	fmt.Println("UpdateLoction. ", location)
	err = c.Update(val, location)
	return err
}
	
func GetLocation(c *mgo.Collection, id int) (Location,error) {
	fmt.Println("GetLocation. ", id)
    result := Location{}
	err := c.Find(bson.M{"id": id}).One(&result)
	if err != nil {
		fmt.Println("Error fetching location.", id)
	}

	fmt.Println(result)
    return result, err
}	

func RemoveLocation(c *mgo.Collection, id int) error {
	result,err := GetLocation(c, id)
	if err != nil {
		return err
	}

	err = c.Remove(result)
    return err
}
//
// Http operations
//
var id int

func HandleAddLocation(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, " ", req.URL)

	decoder := json.NewDecoder(req.Body)
	var loc Location
	err := decoder.Decode(&loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Println(loc)

	loc.Id = id
	id += 1

	lat,lng,err := GetMapsLocation(loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	loc.lat = strconv.FormatFloat(lat, 'f', 6, 64)
	loc.lng = strconv.FormatFloat(lng, 'f', 6, 64)

	c,session,err := OpenCollection("locations")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer session.Close()

	err = InsertLocation(c, loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	out,err := json.Marshal(JSON(loc))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(out)
}

func HandleGetLocation(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, " ", req.URL, " ", mux.Vars(req))

	id,_ := strconv.Atoi(mux.Vars(req)["loc_id"])

	c,session,err := OpenCollection("locations")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()

	loc,err := GetLocation(c, id)
	if err != nil {	
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lat,lng,err := GetMapsLocation(loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	loc.lat = strconv.FormatFloat(lat, 'f', 6, 64)
	loc.lng = strconv.FormatFloat(lng, 'f', 6, 64)


	out,err := json.Marshal(JSON(loc))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Write(out)
}

func HandleUpdateLocation(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, " ", req.URL, " ", mux.Vars(req))
	
	decoder := json.NewDecoder(req.Body)
	var loc Location
	err := decoder.Decode(&loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c,session,err := OpenCollection("locations")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer session.Close()

	rec,err := GetLocation(c, loc.Id)
	if err != nil {	
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if loc.Name != "" 		{ rec.Name = loc.Name }
	if loc.Address != "" 	{ rec.Address = loc.Address }
	if loc.City != ""		{ rec.City = loc.City }
	if loc.State != ""		{ rec.State = loc.State }
	if loc.Zip != ""		{ rec.Zip = loc.Zip }

	err = UpdateLocation(c, rec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lat,lng,err := GetMapsLocation(loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rec.lat = strconv.FormatFloat(lat, 'f', 6, 64)
	rec.lng = strconv.FormatFloat(lng, 'f', 6, 64)

	out,err := json.Marshal(JSON(rec))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusCreated)
	w.Write(out)	
}

func HandleDeleteLocation(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, " ", req.URL, " ", mux.Vars(req))

	id,_ := strconv.Atoi(mux.Vars(req)["loc_id"])

	c,session,err := OpenCollection("locations")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer session.Close()

	err = RemoveLocation(c, id)
	if err != nil {	
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Error(w, "Deleted", http.StatusOK)
}

func HandleError(w http.ResponseWriter, req *http.Request) {
	fmt.Println(req.Method, " ", req.URL)
	http.Error(w, "Invalid request", http.StatusBadRequest)
}

func StartHttpServer(addr string) {
	router := mux.NewRouter()
	router.HandleFunc("/locations/{loc_id:[0-9]+}", HandleGetLocation).Methods("GET")
	router.HandleFunc("/locations", HandleAddLocation).Methods("POST")
	router.HandleFunc("/locations/{loc_id:[0-9]+}", HandleUpdateLocation).Methods("PUT")
	router.HandleFunc("/locations/{loc_id:[0-9]+}", HandleDeleteLocation).Methods("DELETE")
	router.HandleFunc("/", HandleError)

	http.Handle("/", router)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Panic(err)
	}
}


func main() {
	id = 0
	ResetCollection("locations")
	StartHttpServer(":12345")
}   	
	
