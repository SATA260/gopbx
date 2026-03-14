#ifndef GOPBX_INTERNAL_ADAPTER_OUTBOUND_VAD_NATIVE_BRIDGE_H
#define GOPBX_INTERNAL_ADAPTER_OUTBOUND_VAD_NATIVE_BRIDGE_H

#include <stdint.h>
#include <stdlib.h>

#include "onnxruntime_c_api.h"

const OrtApi *GoOrtGetApi(void);
const char *GoOrtGetErrorMessage(const OrtApi *api, OrtStatus *status);
void GoOrtReleaseStatus(const OrtApi *api, OrtStatus *status);

OrtStatus *GoOrtCreateEnv(const OrtApi *api, OrtLoggingLevel log_level, const char *log_id, OrtEnv **env);
void GoOrtReleaseEnv(const OrtApi *api, OrtEnv *env);

OrtStatus *GoOrtCreateSessionOptions(const OrtApi *api, OrtSessionOptions **opts);
void GoOrtReleaseSessionOptions(const OrtApi *api, OrtSessionOptions *opts);
OrtStatus *GoOrtSetIntraOpNumThreads(const OrtApi *api, OrtSessionOptions *opts, int n);
OrtStatus *GoOrtSetInterOpNumThreads(const OrtApi *api, OrtSessionOptions *opts, int n);
OrtStatus *GoOrtSetSessionGraphOptimizationLevel(const OrtApi *api, OrtSessionOptions *opts, GraphOptimizationLevel level);

OrtStatus *GoOrtCreateSession(const OrtApi *api, OrtEnv *env, const char *model_path, OrtSessionOptions *opts, OrtSession **session);
void GoOrtReleaseSession(const OrtApi *api, OrtSession *session);

OrtStatus *GoOrtCreateCpuMemoryInfo(const OrtApi *api, enum OrtAllocatorType alloc_type, enum OrtMemType mem_type, OrtMemoryInfo **info);
void GoOrtReleaseMemoryInfo(const OrtApi *api, OrtMemoryInfo *info);

OrtStatus *GoOrtCreateTensorWithDataAsOrtValue(
    const OrtApi *api,
    const OrtMemoryInfo *info,
    void *data,
    size_t data_len,
    const int64_t *shape,
    size_t shape_len,
    ONNXTensorElementDataType data_type,
    OrtValue **value);
void GoOrtReleaseValue(const OrtApi *api, OrtValue *value);

OrtStatus *GoOrtRun(
    const OrtApi *api,
    OrtSession *session,
    const OrtRunOptions *run_options,
    const char *const *input_names,
    const OrtValue *const *inputs,
    size_t input_len,
    const char *const *output_names,
    size_t output_len,
    OrtValue **outputs);

OrtStatus *GoOrtGetTensorMutableData(const OrtApi *api, OrtValue *value, void **data);

#endif
