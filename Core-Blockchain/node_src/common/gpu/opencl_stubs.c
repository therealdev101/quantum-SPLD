/*
  OpenCL stub implementations for environments without OpenCL kernels.
  These satisfy linker references from gpu_processor.go and allow CUDA-only builds.

  All functions return -1 (failure) so the Go layer will fall back to CPU or CUDA paths.
*/

#include <stddef.h>

int initOpenCL() {
  return -1;
}

int processTxBatchOpenCL(void* txData, int txCount, void* results) {
  (void)txData; (void)txCount; (void)results;
  return -1;
}

int processHashesOpenCL(void* hashes, int count, void* results) {
  (void)hashes; (void)count; (void)results;
  return -1;
}

int verifySignaturesOpenCL(void* signatures, int count, void* results) {
  (void)signatures; (void)count; (void)results;
  return -1;
}

void cleanupOpenCL() {
  // no-op
}
