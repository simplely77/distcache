package distcache

import (
	"fmt"
	"log"
	"reflect"
	"testing"
)

// test Getter and GetterFunc
func TestGetter(t *testing.T){
	var f Getter = GetterFunc(func(key string)([]byte,error){
		return []byte(key),nil
	})
	expect := []byte("key")
	if v,_:=f.Get("key");!reflect.DeepEqual(v,expect){
		t.Errorf("callback failed")
	}
}

var db = map[string]string{
	"Tom":"630",
	"Jack":"589",
	"Sam":"567",
}

// test Get method of Group
func TestGet(t *testing.T){
	loadCounts := make(map[string]int,len(db))
	group := NewGroup("scores",2<<10,GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key",key)
			if v,ok:=db[key];ok{
				loadCounts[key]++
				return []byte(v),nil
			}
			return nil,fmt.Errorf("%s not exist",key)
		}))
	for k,v:=range db{
		if view,err:=group.Get(k);err!=nil||view.String()!=v{
			t.Fatalf("failed to get value of %s",k)
		}
		if _,err:=group.Get(k);err!=nil||loadCounts[k]>1{
			t.Fatalf("cache %s miss",k)
		}
	}
	if view,err:=group.Get("unknown");err==nil{
		t.Fatalf("the value of unknow should be empty,but %s got",view)
	}
}