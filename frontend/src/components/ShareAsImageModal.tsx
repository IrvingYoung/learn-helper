import { useEffect, useState } from "react";
import {
  copyPngToClipboard,
  downloadBlob,
  formatBytes,
  formatFilename,
  getPngDimensions,
  supportsClipboardImageWrite,
} from "../lib/share-as-image";

interface ShareAsImageModalProps {
  open: boolean;
  blob: Blob | null;
  error: string | null;
  slug: string;
  onClose: () => void;
  onRetry: () => void;
}

export function ShareAsImageModal({
  open,
  blob,
  error,
  slug,
  onClose,
  onRetry,
}: ShareAsImageModalProps) {
  const [supportsClipboard, setSupportsClipboard] = useState(false);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const [info, setInfo] = useState<{ size: string; dimensions: string } | null>(null);

  useEffect(() => {
    setSupportsClipboard(supportsClipboardImageWrite());
  }, []);

  useEffect(() => {
    if (!blob) {
      setInfo(null);
      return;
    }
    let cancelled = false;
    setInfo({ size: formatBytes(blob.size), dimensions: "读取中…" });
    getPngDimensions(blob).then((dims) => {
      if (cancelled) return;
      setInfo({
        size: formatBytes(blob.size),
        dimensions: dims ? `${dims.width} × ${dims.height}` : "未知",
      });
    });
    return () => {
      cancelled = true;
    };
  }, [blob]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  useEffect(() => {
    if (!open) {
      setCopyState("idle");
    }
  }, [open]);

  if (!open) return null;

  const handleCopy = async () => {
    if (!blob) return;
    const ok = await copyPngToClipboard(blob);
    setCopyState(ok ? "copied" : "failed");
    if (ok) {
      setTimeout(() => setCopyState((s) => (s === "copied" ? "idle" : s)), 1800);
    }
  };

  const handleDownload = () => {
    if (!blob) return;
    downloadBlob(blob, formatFilename(slug));
  };

  const isLoading = blob === null && error === null;

  return (
    <div
      className="fixed inset-0 bg-black/40 flex items-center justify-center z-50 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="bg-white rounded-lg shadow-xl max-w-3xl w-full max-h-[85vh] flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-3 border-b border-gray-200 shrink-0">
          <h2 className="text-base font-semibold text-gray-900">分享为图片</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 text-2xl leading-none w-8 h-8 flex items-center justify-center rounded transition-colors hover:bg-gray-100"
            aria-label="关闭"
          >
            ×
          </button>
        </div>

        <div className="flex-1 min-h-0 overflow-auto bg-gray-50 p-4 flex items-center justify-center">
          {isLoading ? (
            <div className="flex flex-col items-center gap-3 text-gray-500 text-sm py-12">
              <div className="w-8 h-8 border-2 border-gray-300 border-t-gray-600 rounded-full animate-spin" />
              正在生成图片…
            </div>
          ) : error ? (
            <div className="flex flex-col items-center gap-3 text-center py-12 px-6">
              <div className="w-10 h-10 rounded-full bg-red-50 flex items-center justify-center text-red-500 text-xl">
                !
              </div>
              <p className="text-sm font-medium text-gray-900">生成失败</p>
              <p className="text-xs text-gray-500 max-w-sm">{error}</p>
              <button
                onClick={onRetry}
                className="mt-2 px-4 py-1.5 text-sm bg-gray-900 text-white rounded hover:bg-gray-700 transition-colors"
              >
                重试
              </button>
            </div>
          ) : blob ? (
            <BlobPreview blob={blob} />
          ) : null}
        </div>

        <div className="px-5 py-3 border-t border-gray-200 shrink-0 bg-white">
          {info && !error && !isLoading && (
            <div className="text-xs text-gray-500 mb-3">
              {info.size} · {info.dimensions}
            </div>
          )}
          <div className="flex items-center justify-end gap-2">
            <button
              onClick={onClose}
              className="px-3 py-1.5 text-sm text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded transition-colors"
            >
              关闭
            </button>
            <button
              onClick={handleCopy}
              disabled={!blob || !supportsClipboard}
              title={
                !supportsClipboard
                  ? "当前环境不支持直接复制,请下载后手动粘贴"
                  : undefined
              }
              className="px-3 py-1.5 text-sm border border-gray-300 text-gray-700 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors inline-flex items-center gap-1.5"
            >
              {copyState === "copied" ? (
                <>
                  <span className="text-green-600">✓</span>已复制
                </>
              ) : copyState === "failed" ? (
                "复制失败"
              ) : (
                "复制到剪贴板"
              )}
            </button>
            <button
              onClick={handleDownload}
              disabled={!blob}
              className="px-3 py-1.5 text-sm bg-gray-900 text-white rounded hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              下载 PNG
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

function BlobPreview({ blob }: { blob: Blob }) {
  const [url, setUrl] = useState<string>("");

  useEffect(() => {
    const u = URL.createObjectURL(blob);
    setUrl(u);
    return () => URL.revokeObjectURL(u);
  }, [blob]);

  if (!url) return null;

  return (
    <img
      src={url}
      alt="页面预览"
      className="max-w-full max-h-[60vh] object-contain shadow-md rounded"
    />
  );
}
