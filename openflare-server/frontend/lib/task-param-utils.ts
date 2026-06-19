/**
 * task-param-utils.ts
 *
 * 统一的任务参数 Payload 构建工具。
 *
 * 设计原则：
 *  - 所有 Task Param 的类型转换和空值过滤在这里集中处理，
 *    组件只负责收集原始字符串输入值。
 *  - 新增参数类型时，只需在 coerceParamValue 中增加一个 case。
 *  - 不引入任何框架/UI 依赖，纯工具函数。
 */

import type {TaskParam} from '@/lib/services/admin/types';

/** buildTaskPayload 的返回结构 */
export interface BuildPayloadResult {
  /** 验证通过时返回序列化好的 JSON 字符串；失败时为 null */
  payload: string | null;
  /** 验证失败时返回人类可读的错误信息；成功时为 null */
  error: string | null;
}

/**
 * 根据 TaskParam 的 type 声明，将原始字符串输入值转换为正确的 JSON 值类型。
 *
 * 规则：
 *  - 'number' → 转为 JS number；空字符串视为「未填写」，返回 undefined（由调用方决定是否跳过）
 *  - 'string' / 'text' → 保持字符串，原样返回
 *
 * @returns 转换后的值，或 `undefined` 表示该参数应当从 payload 中省略
 */
function coerceParamValue(
  param: TaskParam,
  rawValue: string,
): { value: string | number | boolean; ok: true } | { value: undefined; ok: true } | { error: string; ok: false } {
  const trimmed = rawValue.trim();

  switch (param.type) {
    case 'number': {
      if (trimmed === '') {
        // 空值 → 省略该字段（对 omitempty 字段友好）
        return { value: undefined, ok: true };
      }
      const num = Number(trimmed);
      if (Number.isNaN(num)) {
        return { error: `「${param.label}」必须是有效的数字`, ok: false };
      }
      return { value: num, ok: true };
    }
    case 'boolean': {
      if (trimmed === '') {
        // 空值 → 省略该字段
        return { value: undefined, ok: true };
      }
      const val = trimmed.toLowerCase();
      if (val === 'true') {
        return { value: true, ok: true };
      } else if (val === 'false') {
        return { value: false, ok: true };
      }
      return { error: `「${param.label}」必须是 true 或 false`, ok: false };
    }
    case 'string':
    case 'text':
    default:
      // 字符串类型：空值也保留（空字符串是合法的字符串值）
      return { value: trimmed, ok: true };
  }
}

/**
 * 将表单收集的原始参数值（全为 string）构建为类型正确的 JSON payload 字符串。
 *
 * 行为：
 *  - `number` 类型参数：序列化为 JSON number，而非 JSON string。
 *  - `number` 类型参数为空且非必填：从 payload 中省略该字段（不发送），
 *    避免后端收到 `"target_type":""` 再尝试解析为 int 时报错。
 *  - `boolean` 类型参数：序列化为 JSON boolean。
 *  - `required` 字段为空：返回 error。
 *
 * @param params   任务的参数定义列表（来自 TaskMeta.params）
 * @param values   表单收集的原始字符串值，key 为 param.name
 * @returns        BuildPayloadResult
 */
export function buildTaskPayload(
  params: TaskParam[],
  values: Record<string, string>,
): BuildPayloadResult {
  const payloadData: Record<string, string | number | boolean> = {};

  for (const param of params) {
    const raw = values[param.name] ?? '';

    // 必填项空值检查
    if (param.required && raw.trim() === '') {
      return { payload: null, error: `「${param.label}」不能为空` };
    }

    const result = coerceParamValue(param, raw);

    if (!result.ok) {
      return { payload: null, error: result.error };
    }

    if (result.value !== undefined) {
      // 有值（包括空字符串）才写入
      payloadData[param.name] = result.value;
    }
    // undefined → 省略该字段（number 类型的空值）
  }

  return { payload: JSON.stringify(payloadData), error: null };
}
