//go:build cgo && gpu

// Forward declarations for CUDA functions (implemented in libsplendor_cuda.a)
extern int cuda_init_device();
extern int cuda_process_hashes(void* input, int count, void* output);
extern int cuda_verify_signatures(void* sigs, void* msgs, void* keys, int count, void* results);
extern int cuda_process_transactions(void* txs, int count, void* results);
extern void cuda_cleanup();

// Include OpenCL implementation
#include "native/opencl_kernels.c"
