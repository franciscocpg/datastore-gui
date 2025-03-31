package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"maps"
	"net/http"
	"strconv"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/julienschmidt/httprouter"
)

var (
	port             = flag.String("port", "", "PORT")
	projectID        = flag.String("projectID", "", "PROJECT_ID")
	dsHost           = flag.String("dsHost", "", "DATASTORE_EMULATOR_HOST")
	responseEntities = []map[string]interface{}{}
	loadEntity       = map[string]interface{}{}
)

func main() {
	flag.Parse()
	router := httprouter.New()
	router.GET("/namespaces", GetNamespaces)
	router.GET("/namespace/:namespace", GetKinds)
	router.GET("/namespace/:namespace/kind/:kind", GetEntities)
	router.GET("/namespace/:namespace/kind/:kind/properties", GetProperties)
	router.DELETE("/namespace/:namespace/kind/:kind", DeleteEntities)
	router.GET("/", Index)
	router.ServeFiles("/index/*filepath", http.Dir("./client/dist"))

	log.Printf("start: PORT=%s, PROJECT_ID=%s, DATASTORE_EMULATOR_HOST=%s", *port, *projectID, *dsHost)
	log.Fatal(http.ListenAndServe(":"+*port, router))
}

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.Redirect(w, r, "/index", 301)
}

type L struct{}

func (l *L) Load(pl []datastore.Property) error {
	loadEntity = load(pl, loadEntity)
	responseEntities = append(responseEntities, loadEntity)

	return nil
}

func load(pl []datastore.Property, dst map[string]interface{}) map[string]interface{} {
	for _, p := range pl {
		switch v := p.Value.(type) {
		case *datastore.Key:
			dst[p.Name] = v.String()
		case *datastore.Entity:
			l := make(map[string]interface{}, 0)
			dst[p.Name] = load(v.Properties, l)
		default:
			dst[p.Name] = p.Value
		}
	}

	return dst
}

func (l *L) LoadKey(k *datastore.Key) error {
	loadEntity = make(map[string]interface{}, 0)
	loadEntity["ID/Name"] = k.String()

	return nil
}

func (l *L) Save() ([]datastore.Property, error) {
	return nil, nil
}

func GetNamespaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	namespaces := []string{"default"}

	res, err := json.Marshal(map[string]interface{}{"namespaces": namespaces})
	if err != nil {
		log.Println(err)
		return
	}

	w.Write(res)
}

func GetKinds(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	kinds := []string{"Creci"}

	res, err := json.Marshal(map[string]interface{}{"kinds": kinds})
	if err != nil {
		log.Println(err)
		return
	}

	w.Write(res)
}

func GetEntities(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, *projectID)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	namespace := ps.ByName("namespace")
	if namespace == "default" {
		namespace = ""
	}

	responseEntities = make([]map[string]interface{}, 0)
	var l []L
	query := datastore.NewQuery(ps.ByName("kind")).Namespace(namespace)
	_, err = client.GetAll(ctx, query, &l)
	if err != nil {
		log.Println(err)
		return
	}

	res, err := json.Marshal(map[string]interface{}{"entities": responseEntities})
	if err != nil {
		log.Println(err)
		return
	}

	w.Write(res)
}

func GetProperties(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, *projectID)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	namespace := ps.ByName("namespace")
	if namespace == "default" {
		namespace = ""
	}

	kind := ps.ByName("kind")
	query := datastore.NewQuery(kind).Namespace(namespace).Limit(1)
	responseEntities = make([]map[string]interface{}, 0)
	var l []L
	_, err = client.GetAll(ctx, query, &l)
	if err != nil {
		log.Println(err)
		return
	}

	keys := maps.Keys(responseEntities[0])
	properties := []string{}
	for k := range keys {
		properties = append(properties, k)
	}

	res, err := json.Marshal(map[string]interface{}{"properties": properties})
	if err != nil {
		log.Println(err)
		return
	}
	w.Write(res)
}

func DeleteEntities(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, *projectID)
	if err != nil {
		log.Println(err)
		return
	}
	defer client.Close()

	namespace := ps.ByName("namespace")
	if namespace == "default" {
		namespace = ""
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		return
	}

	var jsonBody map[string][]string
	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		log.Println(err)
		return
	}

	var keys []*datastore.Key
	for _, v := range jsonBody["keys"] {
		ks := strings.Split(v, "/")[1:]
		var key *datastore.Key
		for i, k := range ks {
			kindKeyValue := strings.Split(k, ",")
			id, err := strconv.ParseInt(kindKeyValue[1], 10, 64)
			if err != nil {
				log.Println(err)
				return
			}

			var name string
			if id == 0 {
				name = kindKeyValue[1]
			}
			kind := kindKeyValue[0]

			if i == 0 {
				if name != "" {
					key = datastore.NameKey(kind, name, nil)
				} else {
					key = datastore.IDKey(kind, id, nil)
				}
			} else {
				if name != "" {
					key = datastore.NameKey(kind, name, key)
				} else {
					key = datastore.IDKey(kind, id, key)
				}
			}
		}

		keys = append(keys, key)
	}

	err = client.DeleteMulti(ctx, keys)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("delete:", keys)
}
