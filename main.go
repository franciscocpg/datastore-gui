package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"

	"cloud.google.com/go/datastore"
	"github.com/julienschmidt/httprouter"
)

var (
	port        = flag.String("port", "", "PORT")
	projectID   = flag.String("projectID", "", "PROJECT_ID")
	dsHost      = flag.String("dsHost", "", "DATASTORE_EMULATOR_HOST")
	entities    = flag.String("entities", "", "ENTITIES")
	isFirestore = false
)

func main() {
	flag.Parse()
	isFirestore = *entities != ""

	router := httprouter.New()
	router.GET("/namespaces", GetNamespaces)
	router.GET("/namespace/:namespace", GetKinds)
	router.GET("/namespace/:namespace/kind/:kind", GetEntities)
	router.GET("/namespace/:namespace/kind/:kind/properties", GetProperties)
	router.DELETE("/namespace/:namespace/kind/:kind", DeleteEntities)
	router.GET("/", Index)
	router.ServeFiles("/index/*filepath", http.Dir("./client/dist"))

	log.Printf("start: PORT=%s, PROJECT_ID=%s, DATASTORE_EMULATOR_HOST=%s, ENTITIES=%s", *port, *projectID, *dsHost, *entities)
	log.Fatal(http.ListenAndServe(":"+*port, router))
}

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	http.Redirect(w, r, "/index", http.StatusMovedPermanently)
}

type L struct {
	loadEntity       map[string]any
	responseEntities []map[string]any
}

func (l *L) Load(pl []datastore.Property) error {
	l.loadEntity = load(pl, l.loadEntity)
	if !slices.ContainsFunc(l.responseEntities, func(e map[string]any) bool {
		return e["ID/Name"] == l.loadEntity["ID/Name"]
	}) {
		l.responseEntities = append(l.responseEntities, l.loadEntity)
	}

	return nil
}

func load(pl []datastore.Property, dst map[string]any) map[string]any {
	for _, p := range pl {
		switch v := p.Value.(type) {
		case *datastore.Key:
			dst[p.Name] = v.String()
		case *datastore.Entity:
			l := make(map[string]any, 0)
			dst[p.Name] = load(v.Properties, l)
		default:
			dst[p.Name] = p.Value
		}
	}

	return dst
}

func (l *L) LoadKey(k *datastore.Key) error {
	l.loadEntity = make(map[string]any, 0)
	l.loadEntity["ID/Name"] = k.String()

	return nil
}

func (l *L) Save() ([]datastore.Property, error) {
	return nil, nil
}

func GetNamespaces(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	namespaces := []string{"default"}

	if !isFirestore {
		ctx := context.Background()
		client, err := datastore.NewClient(ctx, *projectID)
		if err != nil {
			log.Println(err)
			return
		}
		defer client.Close()

		query := datastore.NewQuery("__namespace__").KeysOnly()
		keys, err := client.GetAll(ctx, query, nil)
		if err != nil {
			log.Println(err)
			return
		}

		namespaces = make([]string, 0, len(keys))
		for _, k := range keys {
			if k.Name == "" {
				namespaces = append(namespaces, "default")
				continue
			}
			namespaces = append(namespaces, k.Name)
		}
	}

	res, err := json.Marshal(map[string]any{"namespaces": namespaces})
	if err != nil {
		log.Println(err)
		return
	}

	w.Write(res)
}

func GetKinds(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var kinds []string

	if isFirestore {
		kinds = strings.Split(*entities, ",")
	} else {
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

		query := datastore.NewQuery("__kind__").Namespace(namespace).KeysOnly()
		keys, err := client.GetAll(ctx, query, nil)
		if err != nil {
			log.Println(err)
			return
		}

		kinds = make([]string, 0, len(keys))
		for _, k := range keys {
			if k.Name == "" {
				kinds = append(kinds, "default")
				continue
			}
			kinds = append(kinds, k.Name)
		}
	}

	res, err := json.Marshal(map[string]any{"kinds": kinds})
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

	var l []L
	query := datastore.NewQuery(ps.ByName("kind")).Namespace(namespace).Order("__key__")
	_, err = client.GetAll(ctx, query, &l)
	if err != nil {
		log.Println(err)
		return
	}

	// Collect all entities from all L instances
	var allEntities []map[string]any
	for _, loader := range l {
		allEntities = append(allEntities, loader.responseEntities...)
	}

	res, err := json.Marshal(map[string]any{"entities": allEntities})
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

	properties := []string{"ID/Name"}
	if isFirestore {
		kind := ps.ByName("kind")
		query := datastore.NewQuery(kind).Namespace(namespace).Limit(1)
		var l []L
		_, err = client.GetAll(ctx, query, &l)
		if err != nil {
			log.Println(err)
			return
		}

		// Collect all entities from all L instances
		var allEntities []map[string]any
		for _, loader := range l {
			allEntities = append(allEntities, loader.responseEntities...)
		}

		if len(allEntities) > 0 {
			keys := maps.Keys(allEntities[0])
			for k := range keys {
				if k != "ID/Name" {
					properties = append(properties, k)
				}
			}
		}
	} else {
		query := datastore.NewQuery("__property__").KeysOnly()
		keys, err := client.GetAll(ctx, query, nil)
		if err != nil {
			log.Println(err)
			return
		}

		for _, k := range keys {
			if k.Parent.Name == ps.ByName("kind") {
				name := k.Name
				i := strings.Index(k.Name, ".")
				if i != -1 {
					name = name[:i]
				}

				if properties[len(properties)-1] != name {
					properties = append(properties, name)
				}
			}
		}
	}
	sortedProperties := []string{properties[0]}
	if len(properties) > 1 {
		otherProps := properties[1:]
		sort.Strings(otherProps)
		sortedProperties = append(sortedProperties, otherProps...)
	}

	res, err := json.Marshal(map[string]any{"properties": sortedProperties})
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

	body, err := io.ReadAll(r.Body)
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
			idStr := kindKeyValue[1]
			id, err := strconv.ParseInt(idStr, 10, 64)
			isInt := err == nil

			var name string
			if id == 0 {
				name = idStr
			}
			kind := kindKeyValue[0]

			if i == 0 {
				keys = append(keys, datastore.NameKey(kind, idStr, nil))
				if isInt {
					keys = append(keys, datastore.IDKey(kind, id, nil))
				}
			} else {
				keys = append(keys, datastore.NameKey(kind, name, nil))
				if isInt {
					keys = append(keys, datastore.IDKey(kind, id, key))
				}
			}
		}
	}

	for _, k := range keys {
		err = client.Delete(ctx, k)
		if err != nil {
			log.Println(err)
			return
		}
	}

	log.Println("delete:", keys)
}
