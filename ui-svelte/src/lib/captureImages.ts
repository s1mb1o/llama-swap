// Finds the images that a captured request body carries.

export interface RequestImage {
  /** JSON path of the value, e.g. "messages[0].content[1].image_url.url". */
  path: string;
  /** The data: URL or the http(s) URL. */
  url: string;
  /** MIME type from the data: URL header; "" for a remote URL. */
  mime: string;
  /** Decoded payload size of a data: URL; null for a remote URL. */
  bytes: number | null;
}

const DATA_IMAGE = /^data:(image\/[^;,]+)((?:;[^;,]*)*),/i;
const IMAGE_URL_PATH = /(^|\.)image_url(\.url)?$/;

/**
 * Walk a parsed JSON request body and collect every data:image/... string, plus the
 * http(s) URLs of image_url fields (the OpenAI chat format).
 */
export function findRequestImages(body: unknown): RequestImage[] {
  const found: RequestImage[] = [];
  const walk = (value: unknown, path: string) => {
    if (typeof value === "string") {
      const m = DATA_IMAGE.exec(value);
      if (m) {
        const payload = value.slice(m[0].length);
        found.push({
          path,
          url: value,
          mime: m[1].toLowerCase(),
          bytes: /;base64/i.test(m[2]) ? base64Bytes(payload) : percentBytes(payload),
        });
      } else if (IMAGE_URL_PATH.test(path) && /^https?:\/\//i.test(value)) {
        found.push({ path, url: value, mime: "", bytes: null });
      }
    } else if (Array.isArray(value)) {
      value.forEach((item, i) => walk(item, `${path}[${i}]`));
    } else if (value && typeof value === "object") {
      for (const [key, item] of Object.entries(value)) {
        walk(item, path ? `${path}.${key}` : key);
      }
    }
  };
  walk(body, "");
  return found;
}

/** Decode a data:image/... URL into a Blob. Throws on a malformed payload. */
export function dataUrlToBlob(url: string): Blob {
  const m = DATA_IMAGE.exec(url);
  if (!m) throw new Error("not a data:image URL");
  const payload = url.slice(m[0].length);
  if (/;base64/i.test(m[2])) {
    const binary = atob(payload);
    const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0));
    return new Blob([bytes], { type: m[1] });
  }
  return new Blob([decodeURIComponent(payload)], { type: m[1] });
}

function base64Bytes(payload: string): number {
  const padding = payload.endsWith("==") ? 2 : payload.endsWith("=") ? 1 : 0;
  return Math.floor((payload.length * 3) / 4) - padding;
}

function percentBytes(payload: string): number {
  try {
    return new TextEncoder().encode(decodeURIComponent(payload)).length;
  } catch {
    return payload.length;
  }
}
