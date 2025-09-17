#include <stdint.h>
#include <string.h>

#ifndef HASH_SLOT_BYTES
#define HASH_SLOT_BYTES 256
#endif
#ifndef TX_SLOT_BYTES
#define TX_SLOT_BYTES 1024
#endif
#ifndef HASH_OUTPUT_BYTES
#define HASH_OUTPUT_BYTES 32
#endif
#ifndef TX_RESULT_BYTES
#define TX_RESULT_BYTES 64
#endif
#ifndef SIGNATURE_BYTES
#define SIGNATURE_BYTES 65
#endif
#ifndef MESSAGE_BYTES
#define MESSAGE_BYTES 32
#endif
#ifndef PUBKEY_BYTES
#define PUBKEY_BYTES 65
#endif

// Exported Go helpers implemented in gpu_bridge_exports.go.
extern void go_keccak256(const uint8_t* input, int length, uint8_t* output);
extern int go_verify_signature(const uint8_t* signature, const uint8_t* message, const uint8_t* public_key);
extern int go_process_transaction(const uint8_t* tx, int length, uint8_t* output);

static inline uint32_t clamp_length_opencl(uint32_t length, uint32_t max_length) {
    return length > max_length ? max_length : length;
}

int initOpenCL() {
    // Provide a logical OpenCL device backed by CPU helpers.
    return 1;
}

int processHashesOpenCL(void* hashes, void* lengths, int count, void* results) {
    if (!hashes || !lengths || !results || count <= 0) {
        return -1;
    }

    uint8_t* in = (uint8_t*)hashes;
    uint32_t* lens = (uint32_t*)lengths;
    uint8_t* out = (uint8_t*)results;

    for (int i = 0; i < count; i++) {
        uint32_t length = clamp_length_opencl(lens[i], HASH_SLOT_BYTES);
        uint8_t* item_in = in + ((size_t)i * HASH_SLOT_BYTES);
        uint8_t* item_out = out + ((size_t)i * HASH_OUTPUT_BYTES);
        go_keccak256(item_in, (int)length, item_out);
    }
    return 0;
}

int verifySignaturesOpenCL(void* sigs, void* msgs, void* keys, int count, void* results) {
    if (!sigs || !msgs || !keys || !results || count <= 0) {
        return -1;
    }

    uint8_t* sig_ptr = (uint8_t*)sigs;
    uint8_t* msg_ptr = (uint8_t*)msgs;
    uint8_t* key_ptr = (uint8_t*)keys;
    uint8_t* out_ptr = (uint8_t*)results;

    for (int i = 0; i < count; i++) {
        uint8_t* sig = sig_ptr + ((size_t)i * SIGNATURE_BYTES);
        uint8_t* msg = msg_ptr + ((size_t)i * MESSAGE_BYTES);
        uint8_t* key = key_ptr + ((size_t)i * PUBKEY_BYTES);
        int ok = go_verify_signature(sig, msg, key);
        out_ptr[i] = (uint8_t)(ok ? 1 : 0);
    }
    return 0;
}

int processTxBatchOpenCL(void* txs, void* lengths, int count, void* results) {
    if (!txs || !lengths || !results || count <= 0) {
        return -1;
    }

    uint8_t* tx_ptr = (uint8_t*)txs;
    uint32_t* lens = (uint32_t*)lengths;
    uint8_t* out_ptr = (uint8_t*)results;

    for (int i = 0; i < count; i++) {
        uint8_t* tx = tx_ptr + ((size_t)i * TX_SLOT_BYTES);
        uint32_t length = clamp_length_opencl(lens[i], TX_SLOT_BYTES);
        uint8_t* out = out_ptr + ((size_t)i * TX_RESULT_BYTES);
        int status = go_process_transaction(tx, (int)length, out);
        if (status != 0) {
            return status;
        }
    }
    return 0;
}

void cleanupOpenCL() {
    // Nothing to release in the CPU backed implementation.
}
