package glib

/*
#include <glib.h>
*/
import "C"

type List struct {
	p *C.GList
}

func (l *List) Data() Pointer {
	return Pointer(l.p.data)
}

func (l *List) SetPtr(p Pointer) {
	l.p = (*C.GList)(p)
}

func (l *List) Length() uint {
	return uint(C.g_list_length(l.p))
}

func (l *List) Nth(n uint) Pointer {
	return Pointer(C.g_list_nth_data(l.p, (C.guint)(C.uint(n))))
}

func (l *List) Free() {
	C.g_list_free(l.p)
}
