import { useEffect, useRef, useState } from "react";
import {
  ArrowRight,
  Clock3,
  LockKeyhole,
  ShieldCheck,
} from "lucide-react";

import {
  ViewerRequestError,
  viewerApi,
  type ContentURL,
  type ShareStatus,
  type ViewerSession,
} from "./api";
import "./index.css";

type Phase = "loading" | "locked" | "viewer" | "error";

type ErrorState = {
  title: string;
  message: string;
};

function currentToken(): string {
  const match = window.location.pathname.match(/^\/s\/([^/]+)\/?$/);
  return match ? decodeURIComponent(match[1]) : "";
}

function App() {
  const token = currentToken();
  const [phase, setPhase] = useState<Phase>(token ? "loading" : "error");
  const [status, setStatus] = useState<ShareStatus | null>(null);
  const [session, setSession] = useState<ViewerSession | null>(null);
  const [content, setContent] = useState<ContentURL | null>(null);
  const [pin, setPIN] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<ErrorState | null>(
    token
      ? null
      : {
          title: "链接不完整",
          message: "请重新打开管理员发送的分享链接。",
        },
  );
  const [pinError, setPinError] = useState("");
  const [retryAfter, setRetryAfter] = useState(0);
  const pinRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!token) {
      return;
    }
    let cancelled = false;
    async function load() {
      try {
        const shareStatus = await viewerApi.status(token);
        if (cancelled) return;
        setStatus(shareStatus);
        if (shareStatus.state !== "ACTIVE") {
          setError(
            shareStatus.state === "REVOKED"
              ? { title: "访问已收回", message: "这条分享已被管理员撤销。" }
              : {
                  title: "链接已过期",
                  message: "请联系发送者获取新的访问入口。",
                },
          );
          setPhase("error");
          return;
        }
        try {
          const existing = await viewerApi.session();
          if (cancelled) return;
          if (existing.share_id !== shareStatus.share_id) {
            setPhase("locked");
            return;
          }
          const contentURL = await viewerApi.contentURL();
          if (cancelled) return;
          setSession(existing);
          setContent(contentURL);
          setPhase("viewer");
        } catch (cause) {
          if (cause instanceof ViewerRequestError && cause.status === 401) {
            setPhase("locked");
            return;
          }
          throw cause;
        }
      } catch (cause) {
        if (cancelled) return;
        setError({
          title: "页面暂时无法打开",
          message:
            cause instanceof Error ? cause.message : "请稍后刷新页面再试。",
        });
        setPhase("error");
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (phase !== "viewer") return;
    const heartbeat = window.setInterval(async () => {
      try {
        await viewerApi.heartbeat();
      } catch {
        setError({
          title: "查看会话已结束",
          message: "分享可能已过期或被撤销。",
        });
        setPhase("error");
      }
    }, 45_000);
    return () => window.clearInterval(heartbeat);
  }, [phase]);

  useEffect(() => {
    if (phase !== "viewer") return;
    const refresh = window.setTimeout(async () => {
      try {
        const next = await viewerApi.contentURL();
        setContent(next);
      } catch {
        setError({
          title: "查看会话已结束",
          message: "请重新打开分享并输入密码。",
        });
        setPhase("error");
      }
    }, 8 * 60_000);
    return () => window.clearTimeout(refresh);
  }, [content?.url, phase]);

  useEffect(() => {
    if (phase === "locked") {
      window.setTimeout(() => pinRef.current?.focus(), 180);
    }
  }, [phase]);

  useEffect(() => {
    if (retryAfter <= 0) return;
    const timer = window.setInterval(
      () => setRetryAfter((value) => Math.max(0, value - 1)),
      1000,
    );
    return () => window.clearInterval(timer);
  }, [retryAfter]);

  async function unlock() {
    if (pin.length !== 4 || submitting || retryAfter > 0) return;
    setSubmitting(true);
    setPinError("");
    try {
      await viewerApi.unlock(token, pin);
      const nextSession = await viewerApi.session();
      const nextContent = await viewerApi.contentURL();
      setSession(nextSession);
      setContent(nextContent);
      setPIN("");
      setPhase("viewer");
    } catch (cause) {
      if (cause instanceof ViewerRequestError) {
        if (cause.code === "RATE_LIMITED") {
          setRetryAfter(cause.retryAfter ?? 60);
          setPinError("尝试次数过多，请稍后再试。");
        } else if (cause.status === 401) {
          setPinError("密码不正确，请检查后重新输入。");
        } else {
          setPinError(cause.message);
        }
      } else {
        setPinError("网络连接失败，请稍后重试。");
      }
      setPIN("");
      pinRef.current?.focus();
    } finally {
      setSubmitting(false);
    }
  }

  if (phase === "loading") return <LoadingScreen />;
  if (phase === "error") return <ErrorScreen value={error} />;
  if (phase === "viewer" && session && content) {
    return <ViewerScreen session={session} content={content} status={status} />;
  }

  return (
    <LockScreen
      status={status}
      pin={pin}
      pinError={pinError}
      retryAfter={retryAfter}
      submitting={submitting}
      pinRef={pinRef}
      onPinChange={(value) => {
        setPIN(value);
        setPinError("");
      }}
      onUnlock={unlock}
    />
  );
}

function LockScreen({
  status,
  pin,
  pinError,
  retryAfter,
  submitting,
  pinRef,
  onPinChange,
  onUnlock,
}: {
  status: ShareStatus | null;
  pin: string;
  pinError: string;
  retryAfter: number;
  submitting: boolean;
  pinRef: React.RefObject<HTMLInputElement | null>;
  onPinChange: (value: string) => void;
  onUnlock: () => void;
}) {
  return (
    <main className="lock-page">
      <div className="lock-orbit lock-orbit-a" />
      <div className="lock-orbit lock-orbit-b" />
      <header className="lock-brand">
        <span className="brand-glyph">BB</span>
        <span>
          BelieveBoy
          <small>Private proposal</small>
        </span>
      </header>

      <section className="lock-shell">
        <div className="lock-intro">
          <p className="micro-label">Protected delivery</p>
          <h1>
            这是一份
            <br />
            <em>私密方案。</em>
          </h1>
          <p className="lock-caption">
            输入发送者提供的四位密码。内容不会出现在搜索结果或公开链接中。
          </p>
          <div className="lock-meta">
            <span>
              <ShieldCheck size={15} />
              {status?.type === "PUBLIC" ? "群组分享" : "专属分享"}
            </span>
            {status && (
              <span>
                <Clock3 size={15} />
                有效至 {formatShortDate(status.expires_at)}
              </span>
            )}
          </div>
        </div>

        <div className="unlock-panel">
          <p className="unlock-label">{status?.label ?? "方案访问"}</p>
          <h2>输入访问密码</h2>
          <p className="unlock-help">密码为 4 位数字</p>

          <button
            type="button"
            className="pin-target"
            aria-label="输入四位访问密码"
            onClick={() => pinRef.current?.focus()}
          >
            {[0, 1, 2, 3].map((index) => (
              <span
                key={index}
                data-filled={Boolean(pin[index])}
                data-active={pin.length === index}
              >
                {pin[index] ? "•" : ""}
              </span>
            ))}
          </button>
          <input
            ref={pinRef}
            className="pin-hidden"
            type="text"
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={4}
            value={pin}
            onChange={(event) =>
              onPinChange(event.target.value.replace(/\D/g, "").slice(0, 4))
            }
            onKeyDown={(event) => {
              if (event.key === "Enter") onUnlock();
            }}
          />

          {pinError && (
            <p className="pin-error" role="alert">
              {pinError}
              {retryAfter > 0 ? ` ${retryAfter} 秒后可再次尝试。` : ""}
            </p>
          )}

          <button
            type="button"
            className="unlock-button"
            disabled={pin.length !== 4 || submitting || retryAfter > 0}
            onClick={onUnlock}
          >
            {submitting ? "正在验证" : "进入方案"}
            <ArrowRight size={17} />
          </button>
          <p className="unlock-footnote">
            <LockKeyhole size={13} />
            短时会话 · 禁止公开索引 · 可随时撤销
          </p>
        </div>
      </section>
    </main>
  );
}

function ViewerScreen({
  session,
  content,
  status,
}: {
  session: ViewerSession;
  content: ContentURL;
  status: ShareStatus | null;
}) {
  return (
    <main className="viewer-page">
      <header className="viewer-bar">
        <div className="viewer-title">
          <span className="brand-glyph small">BB</span>
          <span>
            <strong>{session.content_title}</strong>
            <small>
              {status?.label ?? session.label} · {session.watermark_text}
            </small>
          </span>
        </div>
        <div className="viewer-badge">
          <span className="live-dot" />
          私密查看
        </div>
      </header>
      <div className="content-stage">
        <iframe
          key={content.url}
          title={session.content_title}
          src={content.url}
          sandbox="allow-scripts allow-same-origin"
          referrerPolicy="no-referrer"
          className="content-frame"
        />
        <Watermark text={session.watermark_text} />
      </div>
    </main>
  );
}

function Watermark({ text }: { text: string }) {
  return (
    <div className="watermark" aria-hidden="true">
      {Array.from({ length: 18 }, (_, index) => (
        <span key={index}>{text}</span>
      ))}
    </div>
  );
}

function LoadingScreen() {
  return (
    <main className="loading-page">
      <span className="brand-glyph">BB</span>
      <div className="loading-line" />
      <p>正在确认访问权限</p>
    </main>
  );
}

function ErrorScreen({ value }: { value: ErrorState | null }) {
  return (
    <main className="error-page">
      <span className="brand-glyph">BB</span>
      <p className="micro-label">Access unavailable</p>
      <h1>{value?.title ?? "访问不可用"}</h1>
      <p>{value?.message ?? "请重新打开分享链接。"}</p>
    </main>
  );
}

function formatShortDate(value: string): string {
  return new Intl.DateTimeFormat("zh-CN", {
    month: "long",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export default App;
