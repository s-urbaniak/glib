package glib

/*
#include <glib-object.h>
*/
import "C"
import "unsafe"

type ValueArray struct {
	p *C.GValueArray
}

func (va *ValueArray) SetPtr(p Pointer) {
	va.p = (*C.GValueArray)(p)
}

func (va *ValueArray) GetNth(i uint) *Value {
	return (*Value)(C.g_value_array_get_nth(va.p, C.guint(i)))
}

func (va *ValueArray) Len() uint {
	return uint(va.p.n_values)
}

func (va *ValueArray) Unref() {
	C.g_array_unref((*C.GArray)(unsafe.Pointer(va.p)))
}
