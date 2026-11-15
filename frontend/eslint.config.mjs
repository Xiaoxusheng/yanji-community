import { dirname } from "path";
import { fileURLToPath } from "url";
import { FlatCompat } from "@eslint/eslintrc";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const compat = new FlatCompat({ baseDirectory: __dirname });

const eslintConfig = [
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    rules: {
      // 头像/上传图片来自 API 源（/files），使用 <img> 避免 next/image 域名白名单复杂度
      "@next/next/no-img-element": "off",
    },
  },
  { ignores: ["node_modules/**", ".next/**", "out/**", "next-env.d.ts"] },
];

export default eslintConfig;
