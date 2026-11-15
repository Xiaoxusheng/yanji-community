"use client";

// 认证态：/auth/me 查询缓存 + 封装登录/登出失效逻辑。
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./api";
import type { Me } from "./types";

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: () => api.get<Me>("/auth/me"),
    staleTime: 60_000,
  });
}

export function useRequireMe() {
  const { data, isLoading } = useMe();
  return { me: data, isLoading };
}

export function useInvalidateMe() {
  const qc = useQueryClient();
  return () => {
    qc.invalidateQueries({ queryKey: ["me"] });
    qc.invalidateQueries({ queryKey: ["notifications", "unread"] });
  };
}

/** 未读通知数（登录后轮询） */
export function useUnreadCount(enabled: boolean) {
  return useQuery({
    queryKey: ["notifications", "unread"],
    enabled,
    refetchInterval: 30_000,
    queryFn: () => api.get<{ unread: number }>("/me/notifications/unread-count"),
  });
}
