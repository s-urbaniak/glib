package glib

/*
#include <stdlib.h>
#include <glib-object.h>

static inline
GType go_object_type(GObject* o) {
	return G_OBJECT_TYPE(o);
}

typedef struct {
	GClosure cl;
	gpointer o; 
	gulong h_id;
} GoClosure;

typedef struct {
	GoClosure *cl;
	GValue *ret_val;
	guint n_param;
	const GValue *params;
	gpointer ih;
	gpointer data;
} MarshalParams;

extern void go_marshal(gpointer mp);  

static inline
void closure_marshal(GClosure* cl, GValue* ret_val, guint n_param,
		const GValue* params, gpointer ih, gpointer data) {
	MarshalParams mp = {(GoClosure*) cl, ret_val, n_param, params, ih, data};
	go_marshal(&mp);	
}

static inline
GoClosure* go_closure_new(GObject *o) {
	GoClosure *cl = (GoClosure*) g_closure_new_simple(sizeof (GoClosure), NULL);
	cl->o = o;
	g_closure_set_marshal((GClosure *) cl, closure_marshal);
	return cl;
}

static inline
gulong go_signal_connect(GObject* inst, guint sig, GoClosure* cl) {
	return g_signal_connect_closure_by_id(
		inst,
		sig,
		0,
		(GClosure*) cl,
		TRUE
	);
}

static inline
void go_signal_emit(const GValue *inst_and_params, guint sig, GValue *ret) {
	return g_signal_emitv(
		inst_and_params,
		sig,
		0,
		ret
	);
}
*/
import "C"

import (
	"reflect"
	"unsafe"
	"fmt"
)

type SigHandlerId C.gulong

type Object C.GObject

type Params map[string]interface{}

// Returns C pointer
func (o *Object) Ref() *Object {
	return (*Object)(C.g_object_ref(C.gpointer(o)))

}

func (o *Object) Unref() {
	C.g_object_unref(C.gpointer(o))
}

// Returns C pointer
func NewObject(t Type, params Params) *Object {
	if params == nil || len(params) == 0 {
		return (*Object)(C.g_object_newv(C.GType(t), 0, nil))
	}
	p := make([]C.GParameter, len(params))
	i := 0
	for k, v := range params {
		p[i].name = (*C.gchar)(C.CString(k))
		defer C.free(unsafe.Pointer(p[i].name))
		p[i].value = C.GValue(*ValueOf(v))
		i++
	}
	return (*Object)(C.g_object_newv(C.GType(t), C.guint(i), &p[0]))
}

func (o *Object) SetProperty(name string, val interface{}) {
	n := (*C.gchar)(C.CString(name))
	defer C.free(unsafe.Pointer(n))
	C.g_object_set_property((*C.GObject)(o), n, (*C.GValue)(ValueOf(val)))
}

func (o *Object) Type() Type {
	return Type(C.go_object_type((*C.GObject)(o)))
}

func (o *Object) Emit(sig Signal, args ...interface{}) interface{} {
	prms := make([]Value, len(args) + 1)
	prms[0] = *ValueOf(o)
	for i, a := range args {
		prms[i+1] = *ValueOf(a)
	}
	ret := new(Value)
	C.go_signal_emit((*C.GValue)(&prms[0]), C.guint(sig), (*C.GValue)(ret))
	fmt.Println("*** emitl ***")
	return ret.Get()
}

var handlers = map[uintptr]map[SigHandlerId]*reflect.Value{}

func (o *Object) Connect(sig Signal, cb_func interface{}) {
	cb := reflect.ValueOf(cb_func)
	if cb.Kind() != reflect.Func {
		panic("cb_func is not a function")
	}
	// Check that function parameters and return value match to signal
	var sq C.GSignalQuery
	C.g_signal_query(C.guint(sig), &sq)
	ft := cb.Type()
	if ft.NumOut() > 1 || ft.NumOut()==1 && Type(sq.return_type) == TYPE_NONE {
		panic("Number of function return values doesn't match signal spec.")
	}
	if ft.NumIn() != int(sq.n_params) {
		panic("Number of function parameters doesn't match signal spec.")
	}
	if ft.NumOut() != 0 && !Type(sq.return_type).Match(ft.Out(0)) {
		panic("Type of function return value doesn't match signal spec.")
	}
	pt := (*[1<<16]Type)(unsafe.Pointer(sq.param_types))[:int(sq.n_params)]
	for i := 0; i < ft.NumIn(); i++ {
		if !pt[i].Match(ft.In(i)) {
			panic(fmt.Sprintf(
				"%d function parameter type doesn't match signal spec.", i,
			))
		}
	}
	// Setup closure and connect it to signal
	cl := C.go_closure_new((*C.GObject)(o))
	cl.h_id = C.go_signal_connect((*C.GObject)(o), C.guint(sig), cl)
	ptr := o.Uintptr()
	oh := handlers[ptr]
	if oh == nil {
		oh = map[SigHandlerId]*reflect.Value{}
		handlers[ptr] = oh
	}
	oh[SigHandlerId(cl.h_id)] = &cb
}

func (o *Object) Uintptr() uintptr {
	return uintptr(unsafe.Pointer(o))
}

/*typedef struct {
	GClosure *cl;
	GValue *ret_val;
	guint n_param;
	const GValue *params;
	gpointer ih;
	gpointer data;
} MarshalParams;*/

//export go_marshal
func marshal(mp unsafe.Pointer) {
	p := (*C.MarshalParams)(mp)
	fmt.Println("*** marshal ***")
	cl := (*C.GoClosure)(p.cl)
	cb := handlers[uintptr(cl.o)][SigHandlerId(cl.h_id)]

	// TU_SKONCZYLEM
	//cb.Call(in)
	fmt.Println("cb", cb)
	fmt.Println("ret_val", p.ret_val)
}