#ifndef CMUS_WIN_DLFCN_H
#define CMUS_WIN_DLFCN_H
/* Plugins are statically registered. Unix-only X11 integration stays absent. */
#define RTLD_NOW 0
#define RTLD_LAZY 0
static inline void *dlopen(const char *p, int flags) { (void)p; (void)flags; return 0; }
static inline void *dlsym(void *p, const char *name) { (void)p; (void)name; return 0; }
static inline int dlclose(void *p) { (void)p; return 0; }
static inline const char *dlerror(void) { return "Dynamic plugins are unavailable in this static build"; }
#endif
