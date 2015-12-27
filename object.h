#include <stdlib.h>
#include <pthread.h>
#include <glib-object.h>
#include <stdio.h>

static inline
GType _object_type(GObject* o) {
	return G_OBJECT_TYPE(o);
}

typedef struct {
	GClosure cl;
	gulong h_id;
	gboolean no_inst;
} GoClosure;

typedef struct {
	GoClosure *cl;
	GValue *ret_val;
	guint n_param;
	const GValue *params;
	gpointer ih;
	gpointer mr_data;

	pthread_mutex_t mtx;
} MarshalParams;

MarshalParams *_mp = NULL;
pthread_mutex_t _mp_mutex = PTHREAD_MUTEX_INITIALIZER;
pthread_cond_t _mp_cond = PTHREAD_COND_INITIALIZER;

void mp_pass(MarshalParams *mp) {
	// Prelock params mutex.
	pthread_mutex_lock(&mp->mtx);
	// Set global params variable
	pthread_mutex_lock(&_mp_mutex);
	_mp = mp;
	// Signal that _mp is ready
	pthread_cond_broadcast(&_mp_cond);
	pthread_mutex_unlock(&_mp_mutex);

	// Wait for processing
	pthread_mutex_lock(&mp->mtx);
	pthread_mutex_destroy(&mp->mtx);
}

MarshalParams* mp_wait() {
	pthread_mutex_lock(&_mp_mutex);

	while (_mp == NULL) {
        pthread_cond_wait(&_mp_cond, &_mp_mutex);
    }

	// Get params from global variable.
	MarshalParams *mp = _mp;
	// Reset global variable.
	_mp = NULL;
	pthread_mutex_unlock(&_mp_mutex);
	return mp;
}

void mp_processed(MarshalParams* mp) {
	pthread_mutex_unlock(&mp->mtx);
}

static inline
void _object_closure_marshal(GClosure* cl,
							 GValue* ret_val,
							 guint n_param,
							 const GValue* params,
							 gpointer ih,
							 gpointer mr_data) {
	MarshalParams mp = {
		(GoClosure*) cl, ret_val, n_param, params, ih, mr_data,
		PTHREAD_MUTEX_INITIALIZER
	};
	mp_pass(&mp);
}

static inline
GoClosure* _object_closure_new(gboolean no_inst, gpointer p0) {
	GClosure *cl = g_closure_new_simple(sizeof (GoClosure), p0);
	g_closure_set_marshal(cl, _object_closure_marshal);
	GoClosure *gc = (GoClosure*) cl;
	gc->no_inst = no_inst;
	return gc;
}

static inline
gulong _signal_connect(GObject* inst, guint id, GQuark detail, GoClosure* cl) {
	return g_signal_connect_closure_by_id(
		inst,
		id,
		detail,
		(GClosure*) cl,
		TRUE
	);
}

static inline
void _signal_emit(const GValue *inst_and_params, guint id, GQuark detail,
		GValue *ret) {
	return g_signal_emitv( inst_and_params, id, detail, ret);
}
