//go:build cgo && gpu && opencl_stub

/*
  OpenCL stub implementations for environments without OpenCL kernels.
  These satisfy linker references from gpu_processor.go and allow CUDA-only builds.

  All functions return -1 (failure) so the Go layer will fall back to CPU or CUDA paths.
*/

#include <stddef.h>

int initOpenCL() {
  return -1;
}

int processTxBatchOpenCL(void* txData, void* lengths, int txCount, void* results) {
  (void)txData; (void)lengths; (void)txCount; (void)results;
  return -1;
}

int processHashesOpenCL(void* hashes, void* lengths, int count, void* results) {
  (void)hashes; (void)lengths; (void)count; (void)results;
  return -1;
}

int verifySignaturesOpenCL(void* signatures, void* messages, void* keys, int count, void* results) {
  (void)signatures; (void)messages; (void)keys; (void)count; (void)results;
  return -1;
}

void cleanupOpenCL() {
  // no-op
}
