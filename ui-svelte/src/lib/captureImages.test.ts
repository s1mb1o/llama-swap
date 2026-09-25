import { describe, it, expect } from "vitest";
import { findRequestImages, dataUrlToBlob } from "./captureImages";

// 1x1 transparent PNG, 68 bytes.
const PNG =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=";

describe("findRequestImages", () => {
  it("finds a data URL in the OpenAI chat format", () => {
    const body = {
      model: "m",
      messages: [
        {
          role: "user",
          content: [
            { type: "text", text: "what is this?" },
            { type: "image_url", image_url: { url: PNG } },
          ],
        },
      ],
    };
    expect(findRequestImages(body)).toEqual([
      { path: "messages[0].content[1].image_url.url", url: PNG, mime: "image/png", bytes: 68 },
    ]);
  });

  it("finds data URLs in any field and in the string form of image_url", () => {
    const body = { input: [PNG], image_url: PNG.replace("image/png", "image/JPEG") };
    const found = findRequestImages(body);
    expect(found.map((f) => [f.path, f.mime])).toEqual([
      ["input[0]", "image/png"],
      ["image_url", "image/jpeg"],
    ]);
  });

  it("takes http(s) URLs only from image_url fields", () => {
    const body = {
      messages: [
        {
          content: [
            { type: "image_url", image_url: { url: "https://example.com/a.jpg" } },
            { type: "text", text: "https://example.com/b.jpg" },
          ],
        },
      ],
    };
    expect(findRequestImages(body)).toEqual([
      {
        path: "messages[0].content[0].image_url.url",
        url: "https://example.com/a.jpg",
        mime: "",
        bytes: null,
      },
    ]);
  });

  it("ignores text, non-image data URLs and scalars", () => {
    expect(findRequestImages({ a: "data:text/plain;base64,aGk=", b: 1, c: null, d: "x" })).toEqual([]);
    expect(findRequestImages(null)).toEqual([]);
  });

  it("measures a percent-encoded data URL", () => {
    const svg = "data:image/svg+xml,%3Csvg%2F%3E";
    expect(findRequestImages({ svg })[0].bytes).toBe(6);
  });
});

describe("dataUrlToBlob", () => {
  it("decodes a base64 data URL", async () => {
    const blob = dataUrlToBlob(PNG);
    expect(blob.type).toBe("image/png");
    expect(blob.size).toBe(68);
    const head = new Uint8Array(await blob.arrayBuffer()).slice(1, 4);
    expect(new TextDecoder().decode(head)).toBe("PNG");
  });

  it("throws on a non-image URL", () => {
    expect(() => dataUrlToBlob("https://example.com/a.jpg")).toThrow();
  });
});
