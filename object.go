package glib

/*
#include "object.h"
*/
import "C"

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

type ObjectCaster interface {
	AsObject() *Object
}

type Object struct {
	p C.gpointer
}

func (o *Object) g() *C.GObject {
	return (*C.GObject)(o.p)
}

func (o *Object) GetPtr() Pointer {
	return Pointer(o.p)
}

func (o *Object) SetPtr(p Pointer) {
	o.p = C.gpointer(p)
}

func (o *Object) Type() Type {
	return Type(C._object_type(o.g()))
}

func (o *Object) AsObject() *Object {
	return o
}

func (o *Object) Value() *Value {
	v := NewValue(o.Type())
	C.g_value_set_object(v.g(), o.p)
	return v
}

func (o *Object) Ref() *Object {
	r := new(Object)
	r.SetPtr(Pointer(C.g_object_ref(o.p)))
	return r
}

func (o *Object) Unref() {
	C.g_object_unref(C.gpointer(o.p))
}

func (o *Object) RefSink() *Object {
	r := new(Object)
	r.SetPtr(Pointer(C.g_object_ref_sink(o.p)))
	return r
}

func (o *Object) SetProperty(name string, val interface{}) {
	s := C.CString(name)
	defer C.free(unsafe.Pointer(s))
	C.g_object_set_property(o.g(), (*C.gchar)(s),
		ValueOf(val).g())
}

func (o *Object) GetProperty(name string) interface{} {
	s := C.CString(name)
	defer C.free(unsafe.Pointer(s))
	v := new(Value)
	C.g_object_get_property(o.g(), (*C.gchar)(s), v.g())
	return v.Get()
}

func (o *Object) EmitById(sid SignalId, detail Quark, args ...interface{}) interface{} {
	var sq C.GSignalQuery
	C.g_signal_query(C.guint(sid), &sq)
	if len(args) != int(sq.n_params) {
		panic(fmt.Sprintf(
			"*Object.EmitById "+
				"Number of input parameters #%d doesn't match signal spec #%d",
			len(args), int(sq.n_params),
		))
	}
	prms := make([]Value, len(args)+1)
	prms[0] = *ValueOf(o)
	for i, a := range args {
		prms[i+1] = *ValueOf(a)
	}
	ret := new(Value)
	C._signal_emit(prms[0].g(), C.guint(sid), C.GQuark(detail), ret.g())
	return ret.Get()
}

func (o *Object) Emit(sig_name string, args ...interface{}) interface{} {
	sid, detail := SignalLookup(sig_name, o.Type())
	return o.EmitById(sid, detail, args...)
}

type SigHandlerId C.gulong

type sigHandler struct {
	cb, p0 reflect.Value
}

var (
	obj_handlers = make(map[uintptr]map[SigHandlerId]*sigHandler)
	mu           sync.Mutex // protects access to obj_handlers
)

func (o *Object) connect(
	noi bool,
	sid SignalId,
	detail Quark,
	cb_func,
	param0 interface{},
) {
	mu.Lock()
	defer mu.Unlock()

	cb := reflect.ValueOf(cb_func)
	if cb.Kind() != reflect.Func {
		panic("cb_func isn't a function")
	}
	// Check that function parameters and return value match to signal
	var sq C.GSignalQuery

	C.g_signal_query(C.guint(sid), &sq)
	ft := cb.Type()
	if ft.NumOut() > 1 || ft.NumOut() == 1 && Type(sq.return_type) == TYPE_NONE {
		panic("Number of function return values doesn't match signal spec.")
	}
	poffset := 2
	if param0 == nil {
		// There is no param0
		poffset--
	}
	if noi {
		// There is no instance on which signal was emited as first parameter
		poffset--
	} else if !o.Type().Match(ft.In(poffset - 1)) {
		panic(fmt.Sprintf(
			"Callback #%d param. type %s doesn't match signal source: %s",
			poffset-1, ft.In(poffset-1), o.Type(),
		))
	}
	n_params := int(sq.n_params)
	if ft.NumIn() != n_params+poffset {
		panic(fmt.Sprintf(
			"Number of function parameters #%d isn't equal to signal spec: #%d",
			ft.NumIn(), n_params+poffset,
		))
	}
	if ft.NumOut() != 0 && !Type(sq.return_type).Match(ft.Out(0)) {
		panic("Type of function return value doesn't match signal spec.")
	}
	if n_params > 0 {
		pt := (*[1 << 16]Type)(unsafe.Pointer(sq.param_types))[:int(sq.n_params)]
		for i := 0; i < n_params; i++ {
			if !pt[i].Match(ft.In(i + poffset)) {
				panic(fmt.Sprintf(
					"Callback #%d param. type %s doesn't match signal spec %s",
					i+poffset, ft.In(i+poffset), pt[i],
				))
			}
		}
	}
	// Setup closure and connect it to signal
	var gocl *C.GoClosure
	p0 := reflect.ValueOf(param0)
	// Check type of #0 parameter which is set by Connect method
	switch p0.Kind() {
	case reflect.Invalid:
		gocl = C._object_closure_new(gBoolean(noi), nil)
	case reflect.Ptr:
		if !p0.Type().AssignableTo(ft.In(0)) {
			panic(fmt.Sprintf(
				"Callback #0 parameter type: %s doesn't match signal spec: %s",
				ft.In(0), p0.Type(),
			))
		}
		gocl = C._object_closure_new(gBoolean(noi), C.gpointer(p0.Pointer()))
	default:
		panic("Callback parameter #0 isn't a pointer nor nil")
	}
	gocl.h_id = C._signal_connect(o.g(), C.guint(sid), C.GQuark(detail), gocl)
	oh := obj_handlers[uintptr(o.p)]
	if oh == nil {
		oh = make(map[SigHandlerId]*sigHandler)
		obj_handlers[uintptr(o.p)] = oh
	}
	oh[SigHandlerId(gocl.h_id)] = &sigHandler{cb, p0} // p0 for prevent GC
}

// Connect callback to signal specified by id
func (o *Object) ConnectSid(sid SignalId, detail Quark,
	cb_func, param0 interface{}) {
	o.connect(false, sid, detail, cb_func, param0)
}

// Connect callback to signal specified by id.
// Doesn't pass o as first parameter to callback.
func (o *Object) ConnectSidNoi(sid SignalId, detail Quark,
	cb_func, param0 interface{}) {
	o.connect(true, sid, detail, cb_func, param0)
}

// Connect callback to signal specified by name.
func (o *Object) Connect(sig_name string, cb_func, param0 interface{}) {
	sid, detail := SignalLookup(sig_name, o.Type())
	o.connect(false, sid, detail, cb_func, param0)
}

// Connect callback to signal specified by name.
// Doesn't pass o as first parameter to callback.
func (o *Object) ConnectNoi(sig_name string, cb_func, param0 interface{}) {
	sid, detail := SignalLookup(sig_name, o.Type())
	o.connect(true, sid, detail, cb_func, param0)
}

type Params map[string]interface{}

func NewObject(t Type, params Params) *Object {
	if params == nil || len(params) == 0 {
		return &Object{C.g_object_newv(t.g(), 0, nil)}
	}
	p := make([]C.GParameter, len(params))
	i := 0
	for k, v := range params {
		s := C.CString(k)
		defer C.free(unsafe.Pointer(s))
		p[i].name = (*C.gchar)(s)
		p[i].value = *ValueOf(v).g()
		i++
	}
	return &Object{C.g_object_newv(t.g(), C.guint(i), &p[0])}
}

var (
	ptr_t        = reflect.TypeOf(Pointer(nil))
	obj_t        = reflect.TypeOf(&Object{})
	ptr_setter_i = reflect.TypeOf((*PointerSetter)(nil)).Elem()
)

func valueFromPointer(p Pointer, t reflect.Type) reflect.Value {
	v := reflect.New(t).Elem()
	*(*Pointer)(unsafe.Pointer(v.UnsafeAddr())) = p
	return v
}

func convertVal(t reflect.Type, v reflect.Value) reflect.Value {
	var ptr Pointer
	if v.Type() == obj_t {
		ptr = v.Interface().(*Object).GetPtr()
	} else if v.Type() == ptr_t {
		ptr = v.Interface().(Pointer)
	}
	if ptr != nil {
		var ret reflect.Value
		if t.Implements(ptr_setter_i) {
			// Desired type implements PointerSetter so we are creating
			// new value with desired type and set it from ptr
			if t.Kind() == reflect.Ptr {
				ret = reflect.New(t.Elem())
			} else {
				ret = reflect.Zero(t)
			}
			ret.Interface().(PointerSetter).SetPtr(ptr)
		} else if t.Kind() == reflect.Ptr {
			// t doesn't implements PointerSetter but it is pointer
			// so we bypass type checking and setting it from ptr.
			ret = valueFromPointer(ptr, t)
		}
		return ret
	}
	return v
}

func objectMarshal(mp *C.MarshalParams) {
	mu.Lock()
	defer mu.Unlock()

	gc := (*C.GoClosure)(mp.cl)
	n_param := int(mp.n_param)
	first_param := 0
	if gc.no_inst != 0 {
		// Callback without instance on which signal was emited as first param
		first_param++
	}
	prms := (*[1 << 16]Value)(unsafe.Pointer(mp.params))[:n_param]
	var ptr uintptr
	switch p := prms[0].Get().(type) {
	case Pointer:
		ptr = uintptr(p)
	case *Object:
		ptr = uintptr(p.GetPtr())
	default:
		panic(fmt.Sprintf("Unknown type of #1 parameter: %s", prms[0].Type()))
	}
	h := obj_handlers[ptr][SigHandlerId(gc.h_id)]
	prms = prms[first_param:]
	n_param = len(prms)

	if h.p0.Kind() != reflect.Invalid {
		n_param++
	}
	rps := make([]reflect.Value, n_param)
	i := 0
	if h.p0.Kind() != reflect.Invalid {
		// Connect called with param0 != nil
		v := valueFromPointer(Pointer(gc.cl.data), h.p0.Type())
		rps[i] = v
		i++
	}
	cbt := h.cb.Type()
	for _, p := range prms {
		v := reflect.ValueOf(p.Get())
		rps[i] = convertVal(cbt.In(i), v)
		i++
	}
	ret := h.cb.Call(rps)
	if cbt.NumOut() == 1 {
		ret_val := (*Value)(mp.ret_val)
		ret_val.Set(ret[0].Interface())
	}

	// Signal that params were processed
	C.mp_processed(mp)
}

func callbackLoop() {
	for {
		mp := C.mp_wait()
		go objectMarshal(mp)
	}
}

func init() {
	go callbackLoop()
}
