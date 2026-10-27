"use client";

import { useState } from "react";
import { ArrowRight, ShieldCheck } from "lucide-react";

import { adminApi, ApiRequestError } from "@/lib/api";
import { BrandMark } from "@/components/brand-mark";
import { Button } from "@/components/ui/button";
import { Input, Label } from "@/components/ui/field";

export function LoginView({ onSuccess }: { onSuccess: () => void }) {
  const [email, setEmail] = useState("");
  const [credential, setCredential] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    try {
      await adminApi.login(email, credential);
      setCredential("");
      onSuccess();
    } catch (cause) {
      setError(
        cause instanceof ApiRequestError
          ? cause.message
          : "登录失败，请检查网络后重试",
      );
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="relative min-h-dvh overflow-hidden">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -left-24 top-[18%] h-64 w-64 rounded-full border border-ink/8" />
        <div className="absolute -left-16 top-[22%] h-64 w-64 rounded-full border border-ink/6" />
        <div className="absolute bottom-[-8rem] right-[-5rem] h-96 w-96 rounded-full bg-accent-soft/45 blur-3xl" />
      </div>

      <div className="relative mx-auto grid min-h-dvh max-w-[1380px] lg:grid-cols-[1.1fr_.9fr]">
        <section className="flex flex-col justify-between px-6 py-7 sm:px-10 lg:px-14 lg:py-10">
          <BrandMark />
          <div className="my-16 max-w-2xl lg:my-8">
            <h1 className="font-display text-[clamp(3rem,6.4vw,6.5rem)] leading-[0.93] tracking-[-0.045em]">
              把方案交出去，
              <br />
              <span className="text-accent">把控制权留下。</span>
            </h1>
            <p className="mt-8 max-w-md text-[0.95rem] leading-7 text-ink-soft">
              上传自包含 HTML，创建专属或群分享，用四位密码和有效期控制每一次访问。
            </p>
          </div>
          <div className="hidden items-center gap-3 text-[0.74rem] text-muted lg:flex">
            <ShieldCheck size={17} strokeWidth={1.7} />
            <span>独立内容域名 · 短时会话 · 随时撤销</span>
          </div>
        </section>

        <section className="flex items-center border-t border-line bg-paper-deep/55 px-6 py-12 sm:px-10 lg:border-l lg:border-t-0 lg:px-14">
          <div className="w-full max-w-md">
            <p className="text-[0.7rem] uppercase tracking-[0.18em] text-muted">
              Internal access
            </p>
            <h2 className="mt-3 font-display text-[2.4rem] leading-none tracking-[-0.03em]">
              进入工作台
            </h2>
            <p className="mt-4 text-sm leading-6 text-ink-soft">
              这里只对管理员开放。登录状态会保存在安全 Cookie 中。
            </p>

            <form className="mt-9 space-y-5" onSubmit={handleSubmit}>
              <div>
                <Label htmlFor="email">邮箱</Label>
                <Input
                  id="email"
                  type="email"
                  autoComplete="username"
                  placeholder="admin@example.com"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                />
              </div>
              <div>
                <Label htmlFor="credential">密码</Label>
                <Input
                  id="credential"
                  type="password"
                  autoComplete="current-password"
                  placeholder="输入管理员密码"
                  value={credential}
                  onChange={(event) => setCredential(event.target.value)}
                  required
                />
              </div>
              {error && (
                <p role="alert" className="text-[0.8rem] leading-5 text-danger">
                  {error}
                </p>
              )}
              <Button
                type="submit"
                className="w-full !justify-between"
                disabled={pending}
              >
                <span>{pending ? "正在验证…" : "安全登录"}</span>
                <ArrowRight size={17} />
              </Button>
            </form>
          </div>
        </section>
      </div>
    </div>
  );
}
