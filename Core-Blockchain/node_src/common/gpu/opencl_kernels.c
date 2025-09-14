#define CL_TARGET_OPENCL_VERSION 120
#include <CL/cl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

// OpenCL error checking macro
#define CL_CHECK(call) \
    do { \
        cl_int error = call; \
        if (error != CL_SUCCESS) { \
            fprintf(stderr, "OpenCL error at %s:%d - %d\n", __FILE__, __LINE__, error); \
            return -1; \
        } \
    } while(0)

// Global OpenCL context
static cl_platform_id platform = NULL;
static cl_device_id* devices = NULL;
static cl_context context = NULL;
static cl_command_queue* queues = NULL;
static cl_program program = NULL;
static cl_kernel keccak_kernel = NULL;
static cl_kernel ecdsa_kernel = NULL;
static cl_kernel tx_kernel = NULL;
static int device_count = 0;
static bool opencl_initialized = false;

// OpenCL kernel source code
const char* keccak_kernel_source = R"(
__constant ulong keccak_round_constants[24] = {
    0x0000000000000001UL, 0x0000000000008082UL, 0x800000000000808aUL,
    0x8000000080008000UL, 0x000000000000808bUL, 0x0000000080000001UL,
    0x8000000080008081UL, 0x8000000000008009UL, 0x000000000000008aUL,
    0x0000000000000088UL, 0x0000000080008009UL, 0x8000000000008003UL,
    0x8000000000008002UL, 0x8000000000000080UL, 0x000000000000800aUL,
    0x800000008000000aUL, 0x8000000080008081UL, 0x8000000000008080UL,
    0x0000000080000001UL, 0x8000000080008008UL, 0x8000000000000000UL,
    0x0000000080008082UL, 0x800000000000808aUL, 0x8000000080008000UL
};

ulong rotl64(ulong x, int n) {
    return (x << n) | (x >> (64 - n));
}

void keccak_f1600(__private ulong state[25]) {
    ulong C[5], D[5], B[25];
    
    for (int round = 0; round < 24; round++) {
        // Theta step
        for (int x = 0; x < 5; x++) {
            C[x] = state[x] ^ state[x + 5] ^ state[x + 10] ^ state[x + 15] ^ state[x + 20];
        }
        
        for (int x = 0; x < 5; x++) {
            D[x] = C[(x + 4) % 5] ^ rotl64(C[(x + 1) % 5], 1);
        }
        
        for (int x = 0; x < 5; x++) {
            for (int y = 0; y < 5; y++) {
                state[y * 5 + x] ^= D[x];
            }
        }
        
        // Rho and Pi steps
        for (int x = 0; x < 5; x++) {
            for (int y = 0; y < 5; y++) {
                int rho_offset = ((x + 3 * y) % 5) * 5 + x;
                int rho_rotation = ((24 * x + 36 * y) % 64);
                B[y * 5 + ((2 * x + 3 * y) % 5)] = rotl64(state[rho_offset], rho_rotation);
            }
        }
        
        // Chi step
        for (int x = 0; x < 5; x++) {
            for (int y = 0; y < 5; y++) {
                state[y * 5 + x] = B[y * 5 + x] ^ ((~B[y * 5 + ((x + 1) % 5)]) & B[y * 5 + ((x + 2) % 5)]);
            }
        }
        
        // Iota step
        state[0] ^= keccak_round_constants[round];
    }
}

__kernel void keccak256_kernel(__global uchar* input_data, __global int* input_lengths, 
                              __global uchar* output_data, int batch_size) {
    int idx = get_global_id(0);
    
    if (idx >= batch_size) return;
    
    ulong state[25] = {0};
    int input_len = input_lengths[idx];
    __global uchar* input = input_data + idx * 256;
    __global uchar* output = output_data + idx * 32;
    
    // Absorption phase
    int rate = 136; // 1088 bits / 8 = 136 bytes
    int offset = 0;
    
    while (input_len >= rate) {
        for (int i = 0; i < rate / 8; i++) {
            ulong word = 0;
            for (int j = 0; j < 8 && offset + i * 8 + j < input_len; j++) {
                word |= ((ulong)input[offset + i * 8 + j]) << (j * 8);
            }
            state[i] ^= word;
        }
        keccak_f1600(state);
        input_len -= rate;
        offset += rate;
    }
    
    // Final block with padding
    uchar final_block[136] = {0};
    for (int i = 0; i < input_len; i++) {
        final_block[i] = input[offset + i];
    }
    final_block[input_len] = 0x01;
    final_block[rate - 1] |= 0x80;
    
    for (int i = 0; i < rate / 8; i++) {
        ulong word = 0;
        for (int j = 0; j < 8; j++) {
            word |= ((ulong)final_block[i * 8 + j]) << (j * 8);
        }
        state[i] ^= word;
    }
    keccak_f1600(state);
    
    // Squeezing phase
    for (int i = 0; i < 4; i++) {
        for (int j = 0; j < 8; j++) {
            output[i * 8 + j] = (state[i] >> (j * 8)) & 0xFF;
        }
    }
}

__kernel void ecdsa_verify_kernel(__global uchar* signatures, __global uchar* messages, 
                                 __global uchar* public_keys, __global uchar* results, int batch_size) {
    int idx = get_global_id(0);
    
    if (idx >= batch_size) return;
    
    __global uchar* sig = signatures + idx * 65;
    __global uchar* msg = messages + idx * 32;
    __global uchar* pubkey = public_keys + idx * 64;
    
    // Simplified verification for demonstration
    uchar valid = 1;
    for (int i = 0; i < 32 && valid; i++) {
        if (msg[i] == 0 && sig[i] == 0) {
            valid = 0;
        }
    }
    
    results[idx] = valid;
}

__kernel void transaction_process_kernel(__global uchar* tx_data, __global int* tx_lengths, 
                                       __global uchar* results, int batch_size) {
    int idx = get_global_id(0);
    
    if (idx >= batch_size) return;
    
    __global uchar* tx = tx_data + idx * 1024;
    int tx_len = tx_lengths[idx];
    __global uchar* result = results + idx * 64;
    
    // Simple hash of transaction
    ulong state[25] = {0};
    
    for (int i = 0; i < tx_len && i < 136; i += 8) {
        ulong word = 0;
        for (int j = 0; j < 8 && i + j < tx_len; j++) {
            word |= ((ulong)tx[i + j]) << (j * 8);
        }
        state[i / 8] ^= word;
    }
    
    keccak_f1600(state);
    
    // Output hash
    for (int i = 0; i < 4; i++) {
        for (int j = 0; j < 8; j++) {
            result[i * 8 + j] = (state[i] >> (j * 8)) & 0xFF;
        }
    }
    
    // Set validity flag
    result[32] = (tx_len > 0) ? 1 : 0;
}
)";

// Host functions
int initOpenCL() {
    if (opencl_initialized) {
        return device_count;
    }
    
    cl_uint platform_count;
    CL_CHECK(clGetPlatformIDs(0, NULL, &platform_count));
    
    if (platform_count == 0) {
        fprintf(stderr, "No OpenCL platforms found\n");
        return -1;
    }
    
    cl_platform_id* platforms = malloc(platform_count * sizeof(cl_platform_id));
    CL_CHECK(clGetPlatformIDs(platform_count, platforms, NULL));
    
    // Use first platform
    platform = platforms[0];
    free(platforms);
    
    // Get GPU devices
    cl_uint gpu_count;
    cl_int err = clGetDeviceIDs(platform, CL_DEVICE_TYPE_GPU, 0, NULL, &gpu_count);
    if (err != CL_SUCCESS || gpu_count == 0) {
        // Fallback to CPU devices
        CL_CHECK(clGetDeviceIDs(platform, CL_DEVICE_TYPE_CPU, 0, NULL, &gpu_count));
    }
    
    device_count = gpu_count;
    devices = malloc(device_count * sizeof(cl_device_id));
    
    err = clGetDeviceIDs(platform, CL_DEVICE_TYPE_GPU, device_count, devices, NULL);
    if (err != CL_SUCCESS) {
        CL_CHECK(clGetDeviceIDs(platform, CL_DEVICE_TYPE_CPU, device_count, devices, NULL));
    }
    
    // Create context
    context = clCreateContext(NULL, device_count, devices, NULL, NULL, &err);
    CL_CHECK(err);
    
    // Create command queues
    queues = malloc(device_count * sizeof(cl_command_queue));
    for (int i = 0; i < device_count; i++) {
#ifdef CL_VERSION_2_0
        cl_queue_properties properties[] = {0};
        queues[i] = clCreateCommandQueueWithProperties(context, devices[i], properties, &err);
#else
        queues[i] = clCreateCommandQueue(context, devices[i], 0, &err);
#endif
        CL_CHECK(err);
    }
    
    // Create and build program
    program = clCreateProgramWithSource(context, 1, &keccak_kernel_source, NULL, &err);
    CL_CHECK(err);
    
    err = clBuildProgram(program, device_count, devices, NULL, NULL, NULL);
    if (err != CL_SUCCESS) {
        size_t log_size;
        clGetProgramBuildInfo(program, devices[0], CL_PROGRAM_BUILD_LOG, 0, NULL, &log_size);
        char* log = malloc(log_size);
        clGetProgramBuildInfo(program, devices[0], CL_PROGRAM_BUILD_LOG, log_size, log, NULL);
        fprintf(stderr, "OpenCL build error: %s\n", log);
        free(log);
        return -1;
    }
    
    // Create kernels
    keccak_kernel = clCreateKernel(program, "keccak256_kernel", &err);
    CL_CHECK(err);
    
    ecdsa_kernel = clCreateKernel(program, "ecdsa_verify_kernel", &err);
    CL_CHECK(err);
    
    tx_kernel = clCreateKernel(program, "transaction_process_kernel", &err);
    CL_CHECK(err);
    
    opencl_initialized = true;
    printf("OpenCL initialized with %d devices\n", device_count);
    return device_count;
}

int processHashesOpenCL(void* hashes, int count, void* results) {
    if (!opencl_initialized || count <= 0) {
        return -1;
    }
    
    cl_int err;
    
    // Create buffers
    size_t input_size = count * 256;
    size_t output_size = count * 32;
    
    cl_mem input_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, input_size, NULL, &err);
    CL_CHECK(err);
    
    cl_mem lengths_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, count * sizeof(int), NULL, &err);
    CL_CHECK(err);
    
    cl_mem output_buffer = clCreateBuffer(context, CL_MEM_WRITE_ONLY, output_size, NULL, &err);
    CL_CHECK(err);
    
    // Copy input data
    CL_CHECK(clEnqueueWriteBuffer(queues[0], input_buffer, CL_TRUE, 0, input_size, hashes, 0, NULL, NULL));
    
    // Set lengths (assuming 32 bytes per input)
    int* lengths = malloc(count * sizeof(int));
    for (int i = 0; i < count; i++) {
        lengths[i] = 32;
    }
    CL_CHECK(clEnqueueWriteBuffer(queues[0], lengths_buffer, CL_TRUE, 0, count * sizeof(int), lengths, 0, NULL, NULL));
    
    // Set kernel arguments
    CL_CHECK(clSetKernelArg(keccak_kernel, 0, sizeof(cl_mem), &input_buffer));
    CL_CHECK(clSetKernelArg(keccak_kernel, 1, sizeof(cl_mem), &lengths_buffer));
    CL_CHECK(clSetKernelArg(keccak_kernel, 2, sizeof(cl_mem), &output_buffer));
    CL_CHECK(clSetKernelArg(keccak_kernel, 3, sizeof(int), &count));
    
    // Execute kernel
    size_t global_work_size = count;
    size_t local_work_size = 256;
    if (global_work_size % local_work_size != 0) {
        global_work_size = ((count + local_work_size - 1) / local_work_size) * local_work_size;
    }
    
    CL_CHECK(clEnqueueNDRangeKernel(queues[0], keccak_kernel, 1, NULL, &global_work_size, &local_work_size, 0, NULL, NULL));
    
    // Read results
    CL_CHECK(clEnqueueReadBuffer(queues[0], output_buffer, CL_TRUE, 0, output_size, results, 0, NULL, NULL));
    
    // Cleanup
    clReleaseMemObject(input_buffer);
    clReleaseMemObject(lengths_buffer);
    clReleaseMemObject(output_buffer);
    free(lengths);
    
    return 0;
}

int verifySignaturesOpenCL(void* signatures, int count, void* results) {
    if (!opencl_initialized || count <= 0) {
        return -1;
    }
    
    cl_int err;
    
    // Create buffers
    size_t sig_size = count * 65;
    size_t msg_size = count * 32;
    size_t key_size = count * 64;
    
    cl_mem sig_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, sig_size, NULL, &err);
    CL_CHECK(err);
    
    cl_mem msg_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, msg_size, NULL, &err);
    CL_CHECK(err);
    
    cl_mem key_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, key_size, NULL, &err);
    CL_CHECK(err);
    
    cl_mem result_buffer = clCreateBuffer(context, CL_MEM_WRITE_ONLY, count, NULL, &err);
    CL_CHECK(err);
    
    // Copy input data (signatures + messages + pubkeys may be packed in a single buffer)
    // If 'signatures' points to a packed layout [65 | 32 | 64] * count, deinterleave into separate buffers.
    uint8_t* packed = (uint8_t*)signatures;
    uint8_t* host_sigs = (uint8_t*)malloc(sig_size);
    uint8_t* host_msgs = (uint8_t*)malloc(msg_size);
    uint8_t* host_keys = (uint8_t*)malloc(key_size);
    if (host_sigs == NULL || host_msgs == NULL || host_keys == NULL) {
        if (host_sigs) free(host_sigs);
        if (host_msgs) free(host_msgs);
        if (host_keys) free(host_keys);
        return -1;
    }

    size_t stride = 65 + 32 + 64;
    for (int i = 0; i < count; i++) {
        memcpy(host_sigs + i * 65, packed + i * stride, 65);
        memcpy(host_msgs + i * 32, packed + i * stride + 65, 32);
        memcpy(host_keys + i * 64, packed + i * stride + 65 + 32, 64);
    }

    CL_CHECK(clEnqueueWriteBuffer(queues[0], sig_buffer, CL_TRUE, 0, sig_size, host_sigs, 0, NULL, NULL));
    CL_CHECK(clEnqueueWriteBuffer(queues[0], msg_buffer, CL_TRUE, 0, msg_size, host_msgs, 0, NULL, NULL));
    CL_CHECK(clEnqueueWriteBuffer(queues[0], key_buffer, CL_TRUE, 0, key_size, host_keys, 0, NULL, NULL));
    
    // Set kernel arguments
    CL_CHECK(clSetKernelArg(ecdsa_kernel, 0, sizeof(cl_mem), &sig_buffer));
    CL_CHECK(clSetKernelArg(ecdsa_kernel, 1, sizeof(cl_mem), &msg_buffer));
    CL_CHECK(clSetKernelArg(ecdsa_kernel, 2, sizeof(cl_mem), &key_buffer));
    CL_CHECK(clSetKernelArg(ecdsa_kernel, 3, sizeof(cl_mem), &result_buffer));
    CL_CHECK(clSetKernelArg(ecdsa_kernel, 4, sizeof(int), &count));
    
    // Execute kernel
    size_t global_work_size = count;
    size_t local_work_size = 256;
    if (global_work_size % local_work_size != 0) {
        global_work_size = ((count + local_work_size - 1) / local_work_size) * local_work_size;
    }
    
    CL_CHECK(clEnqueueNDRangeKernel(queues[0], ecdsa_kernel, 1, NULL, &global_work_size, &local_work_size, 0, NULL, NULL));
    
    // Read results
    CL_CHECK(clEnqueueReadBuffer(queues[0], result_buffer, CL_TRUE, 0, count, results, 0, NULL, NULL));
    
    // Cleanup
    clReleaseMemObject(sig_buffer);
    clReleaseMemObject(msg_buffer);
    clReleaseMemObject(key_buffer);
    clReleaseMemObject(result_buffer);

    // Free host-side temporary buffers
    free(host_sigs);
    free(host_msgs);
    free(host_keys);
    
    return 0;
}

int processTxBatchOpenCL(void* txData, int txCount, void* results) {
    if (!opencl_initialized || txCount <= 0) {
        return -1;
    }
    
    cl_int err;
    
    // Create buffers
    size_t tx_data_size = txCount * 1024;
    size_t result_size = txCount * 64;
    
    cl_mem tx_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, tx_data_size, NULL, &err);
    CL_CHECK(err);
    
    cl_mem lengths_buffer = clCreateBuffer(context, CL_MEM_READ_ONLY, txCount * sizeof(int), NULL, &err);
    CL_CHECK(err);
    
    cl_mem result_buffer = clCreateBuffer(context, CL_MEM_WRITE_ONLY, result_size, NULL, &err);
    CL_CHECK(err);
    
    // Copy input data
    CL_CHECK(clEnqueueWriteBuffer(queues[0], tx_buffer, CL_TRUE, 0, tx_data_size, txData, 0, NULL, NULL));
    
    // Set lengths
    int* lengths = malloc(txCount * sizeof(int));
    for (int i = 0; i < txCount; i++) {
        lengths[i] = 100; // Simplified
    }
    CL_CHECK(clEnqueueWriteBuffer(queues[0], lengths_buffer, CL_TRUE, 0, txCount * sizeof(int), lengths, 0, NULL, NULL));
    
    // Set kernel arguments
    CL_CHECK(clSetKernelArg(tx_kernel, 0, sizeof(cl_mem), &tx_buffer));
    CL_CHECK(clSetKernelArg(tx_kernel, 1, sizeof(cl_mem), &lengths_buffer));
    CL_CHECK(clSetKernelArg(tx_kernel, 2, sizeof(cl_mem), &result_buffer));
    CL_CHECK(clSetKernelArg(tx_kernel, 3, sizeof(int), &txCount));
    
    // Execute kernel
    size_t global_work_size = txCount;
    size_t local_work_size = 256;
    if (global_work_size % local_work_size != 0) {
        global_work_size = ((txCount + local_work_size - 1) / local_work_size) * local_work_size;
    }
    
    CL_CHECK(clEnqueueNDRangeKernel(queues[0], tx_kernel, 1, NULL, &global_work_size, &local_work_size, 0, NULL, NULL));
    
    // Read results
    CL_CHECK(clEnqueueReadBuffer(queues[0], result_buffer, CL_TRUE, 0, result_size, results, 0, NULL, NULL));
    
    // Cleanup
    clReleaseMemObject(tx_buffer);
    clReleaseMemObject(lengths_buffer);
    clReleaseMemObject(result_buffer);
    free(lengths);
    
    return 0;
}

void cleanupOpenCL() {
    if (!opencl_initialized) {
        return;
    }
    
    if (keccak_kernel) clReleaseKernel(keccak_kernel);
    if (ecdsa_kernel) clReleaseKernel(ecdsa_kernel);
    if (tx_kernel) clReleaseKernel(tx_kernel);
    if (program) clReleaseProgram(program);
    
    if (queues) {
        for (int i = 0; i < device_count; i++) {
            clReleaseCommandQueue(queues[i]);
        }
        free(queues);
    }
    
    if (context) clReleaseContext(context);
    if (devices) free(devices);
    
    opencl_initialized = false;
    device_count = 0;
    
    printf("OpenCL cleanup completed\n");
}

#ifdef __cplusplus
}
#endif
