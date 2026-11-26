"use client";

// 基于 lucide-react 的统一设计级矢量图标封装（保持全站既有 Icon* 命名 100% 兼容，统一 1.75px 光学描边）。
import type { LucideProps } from "lucide-react";
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  Award,
  BarChart3,
  Bell,
  BellOff,
  Bookmark,
  Calendar,
  Check,
  CheckCheck,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Columns2,
  Command,
  Compass,
  Copy,
  CornerDownLeft,
  Edit3,
  ExternalLink,
  Eye,
  EyeOff,
  FileText,
  Flag,
  Flame,
  Hash,
  HelpCircle,
  History,
  Home,
  Image as LucideImage,
  Inbox,
  LayoutGrid,
  Link2,
  Lock,
  LogOut,
  MapPin,
  MessageCircle,
  Moon,
  Pin,
  Plus,
  RefreshCw,
  Search,
  Send,
  Settings,
  Share2,
  Shield,
  SlidersHorizontal,
  Sparkles,
  Sun,
  ThumbsUp,
  Trash2,
  User,
  X,
} from "lucide-react";

export type IconProps = LucideProps & { size?: number };

const base = (p: IconProps): LucideProps => ({
  size: p.size ?? 16,
  strokeWidth: p.strokeWidth ?? 1.75,
  "aria-hidden": true,
  ...p,
});

export function IconHome(p: IconProps) {
  return <Home {...base(p)} />;
}
export function IconGrid(p: IconProps) {
  return <LayoutGrid {...base(p)} />;
}
export function IconCalendar(p: IconProps) {
  return <Calendar {...base(p)} />;
}
export function IconSearch(p: IconProps) {
  return <Search {...base(p)} />;
}
export function IconBell(p: IconProps) {
  return <Bell {...base(p)} />;
}
export function IconBellOff(p: IconProps) {
  return <BellOff {...base(p)} />;
}
export function IconMessage(p: IconProps) {
  return <MessageCircle {...base(p)} />;
}
export function IconPlus(p: IconProps) {
  return <Plus {...base(p)} />;
}
export function IconThumbUp(p: IconProps) {
  return <ThumbsUp {...base(p)} />;
}
export function IconBookmark(p: IconProps) {
  return <Bookmark {...base(p)} />;
}
export function IconEye(p: IconProps) {
  return <Eye {...base(p)} />;
}
export function IconShare(p: IconProps) {
  return <Share2 {...base(p)} />;
}
export function IconPin(p: IconProps) {
  return <Pin {...base(p)} />;
}
export function IconAward(p: IconProps) {
  return <Award {...base(p)} />;
}
export function IconCheck(p: IconProps) {
  return <Check {...base(p)} />;
}
export function IconCheckCircle(p: IconProps) {
  return <CheckCircle2 {...base(p)} />;
}
export function IconCheckCheck(p: IconProps) {
  return <CheckCheck {...base(p)} />;
}
export function IconAlert(p: IconProps) {
  return <AlertCircle {...base(p)} />;
}
export function IconInbox(p: IconProps) {
  return <Inbox {...base(p)} />;
}
export function IconX(p: IconProps) {
  return <X {...base(p)} />;
}
export function IconSun(p: IconProps) {
  return <Sun {...base(p)} />;
}
export function IconMoon(p: IconProps) {
  return <Moon {...base(p)} />;
}
export function IconUser(p: IconProps) {
  return <User {...base(p)} />;
}
export function IconSettings(p: IconProps) {
  return <Settings {...base(p)} />;
}
export function IconEdit(p: IconProps) {
  return <Edit3 {...base(p)} />;
}
export function IconTrash(p: IconProps) {
  return <Trash2 {...base(p)} />;
}
export function IconFlag(p: IconProps) {
  return <Flag {...base(p)} />;
}
export function IconHistory(p: IconProps) {
  return <History {...base(p)} />;
}
export function IconLock(p: IconProps) {
  return <Lock {...base(p)} />;
}
export function IconImage(p: IconProps) {
  return <LucideImage {...base(p)} />;
}
export function IconLink(p: IconProps) {
  return <Link2 {...base(p)} />;
}
export function IconFileText(p: IconProps) {
  return <FileText {...base(p)} />;
}
export function IconHelpCircle(p: IconProps) {
  return <HelpCircle {...base(p)} />;
}
export function IconBarChart(p: IconProps) {
  return <BarChart3 {...base(p)} />;
}
export function IconShield(p: IconProps) {
  return <Shield {...base(p)} />;
}
export function IconArrowLeft(p: IconProps) {
  return <ArrowLeft {...base(p)} />;
}
export function IconArrowRight(p: IconProps) {
  return <ArrowRight {...base(p)} />;
}
export function IconArrowUp(p: IconProps) {
  return <ArrowUp {...base(p)} />;
}
export function IconLogOut(p: IconProps) {
  return <LogOut {...base(p)} />;
}
export function IconSliders(p: IconProps) {
  return <SlidersHorizontal {...base(p)} />;
}
export function IconColumns(p: IconProps) {
  return <Columns2 {...base(p)} />;
}
export function IconMapPin(p: IconProps) {
  return <MapPin {...base(p)} />;
}
export function IconHash(p: IconProps) {
  return <Hash {...base(p)} />;
}
export function IconEyeOff(p: IconProps) {
  return <EyeOff {...base(p)} />;
}
export function IconRefreshCw(p: IconProps) {
  return <RefreshCw {...base(p)} />;
}
export function IconSparkles(p: IconProps) {
  return <Sparkles {...base(p)} />;
}
export function IconFlame(p: IconProps) {
  return <Flame {...base(p)} />;
}
export function IconSend(p: IconProps) {
  return <Send {...base(p)} />;
}
export function IconCommand(p: IconProps) {
  return <Command {...base(p)} />;
}
export function IconCornerDownLeft(p: IconProps) {
  return <CornerDownLeft {...base(p)} />;
}
export function IconCompass(p: IconProps) {
  return <Compass {...base(p)} />;
}
export function IconClock(p: IconProps) {
  return <Clock {...base(p)} />;
}
export function IconChevronDown(p: IconProps) {
  return <ChevronDown {...base(p)} />;
}
export function IconChevronRight(p: IconProps) {
  return <ChevronRight {...base(p)} />;
}
export function IconExternalLink(p: IconProps) {
  return <ExternalLink {...base(p)} />;
}
export function IconCopy(p: IconProps) {
  return <Copy {...base(p)} />;
}
