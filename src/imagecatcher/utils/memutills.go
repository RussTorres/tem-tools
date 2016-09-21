package utils

//#include <stdlib.h>
import "C"
import (
	"unsafe"
)

// Alloc allocate a buffer of the requested size using C.malloc
func Alloc(size uint32) (unsafe.Pointer, []byte) {
	bytearray := C.malloc(C.size_t(size))
	bytearrayPtr := unsafe.Pointer(bytearray)
	bytes := C.GoBytes(bytearrayPtr, C.int(size))
	return bytearrayPtr, bytes
}

// Free frees a buffer allocated with C.malloc using C.free
func Free(cptr unsafe.Pointer) {
	if cptr != nil {
		C.free(cptr)
	}
}
