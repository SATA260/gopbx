#include "native_bridge.h"

const OrtApi *GoOrtGetApi(void) {
	return OrtGetApiBase()->GetApi(ORT_API_VERSION);
}

const char *GoOrtGetErrorMessage(const OrtApi *api, OrtStatus *status) {
	return api->GetErrorMessage(status);
}

void GoOrtReleaseStatus(const OrtApi *api, OrtStatus *status) {
	if (status != NULL) {
		api->ReleaseStatus(status);
	}
}

OrtStatus *GoOrtCreateEnv(const OrtApi *api, OrtLoggingLevel log_level, const char *log_id, OrtEnv **env) {
	return api->CreateEnv(log_level, log_id, env);
}

void GoOrtReleaseEnv(const OrtApi *api, OrtEnv *env) {
	if (env != NULL) {
		api->ReleaseEnv(env);
	}
}

OrtStatus *GoOrtCreateSessionOptions(const OrtApi *api, OrtSessionOptions **opts) {
	return api->CreateSessionOptions(opts);
}

void GoOrtReleaseSessionOptions(const OrtApi *api, OrtSessionOptions *opts) {
	if (opts != NULL) {
		api->ReleaseSessionOptions(opts);
	}
}

OrtStatus *GoOrtSetIntraOpNumThreads(const OrtApi *api, OrtSessionOptions *opts, int n) {
	return api->SetIntraOpNumThreads(opts, n);
}

OrtStatus *GoOrtSetInterOpNumThreads(const OrtApi *api, OrtSessionOptions *opts, int n) {
	return api->SetInterOpNumThreads(opts, n);
}

OrtStatus *GoOrtSetSessionGraphOptimizationLevel(const OrtApi *api, OrtSessionOptions *opts, GraphOptimizationLevel level) {
	return api->SetSessionGraphOptimizationLevel(opts, level);
}

OrtStatus *GoOrtCreateSession(const OrtApi *api, OrtEnv *env, const char *model_path, OrtSessionOptions *opts, OrtSession **session) {
	return api->CreateSession(env, model_path, opts, session);
}

void GoOrtReleaseSession(const OrtApi *api, OrtSession *session) {
	if (session != NULL) {
		api->ReleaseSession(session);
	}
}

OrtStatus *GoOrtCreateCpuMemoryInfo(const OrtApi *api, enum OrtAllocatorType alloc_type, enum OrtMemType mem_type, OrtMemoryInfo **info) {
	return api->CreateCpuMemoryInfo(alloc_type, mem_type, info);
}

void GoOrtReleaseMemoryInfo(const OrtApi *api, OrtMemoryInfo *info) {
	if (info != NULL) {
		api->ReleaseMemoryInfo(info);
	}
}

OrtStatus *GoOrtCreateTensorWithDataAsOrtValue(
	const OrtApi *api,
	const OrtMemoryInfo *info,
	void *data,
	size_t data_len,
	const int64_t *shape,
	size_t shape_len,
	ONNXTensorElementDataType data_type,
	OrtValue **value) {
	return api->CreateTensorWithDataAsOrtValue(info, data, data_len, shape, shape_len, data_type, value);
}

void GoOrtReleaseValue(const OrtApi *api, OrtValue *value) {
	if (value != NULL) {
		api->ReleaseValue(value);
	}
}

OrtStatus *GoOrtRun(
	const OrtApi *api,
	OrtSession *session,
	const OrtRunOptions *run_options,
	const char *const *input_names,
	const OrtValue *const *inputs,
	size_t input_len,
	const char *const *output_names,
	size_t output_len,
	OrtValue **outputs) {
	return api->Run(session, run_options, input_names, inputs, input_len, output_names, output_len, outputs);
}

OrtStatus *GoOrtGetTensorMutableData(const OrtApi *api, OrtValue *value, void **data) {
	return api->GetTensorMutableData(value, data);
}
