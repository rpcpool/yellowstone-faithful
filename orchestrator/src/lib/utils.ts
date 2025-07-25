import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";
import { IndexType } from "./epochs";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function indexTypeToKebabCase(indexType: IndexType): string {
  return indexType
    .replace(/([A-Z])/g, '-$1')
    .toLowerCase()
    .replace(/^-/, ''); // Remove leading dash
}

export function humanizeSize(bytes: number | string | bigint): string {
  const numBytes = typeof bytes === 'bigint' ? Number(bytes) : Number(bytes);
  
  if (numBytes === 0) return '0 B';
  
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(numBytes) / Math.log(k));
  
  return `${parseFloat((numBytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}
