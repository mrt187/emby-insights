import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Emby Insights — Dein Medien-Dashboard",
  description: "Dein persönlicher Überblick über deine Mediathek.",
  openGraph: {
    title: "Emby Insights",
    description: "Dein persönlicher Überblick über deine Mediathek.",
    images: ["/og.webp"],
  },
  twitter: {
    card: "summary_large_image",
    title: "Emby Insights",
    description: "Dein persönlicher Überblick über deine Mediathek.",
    images: ["/og.webp"],
  },
  icons: {
    icon: [{ url: "/emby-insights-logo.svg", type: "image/svg+xml" }],
    shortcut: "/emby-insights-logo.svg",
    apple: "/emby-insights-logo.png",
  },
};

export const viewport = {
  colorScheme: "dark",
  themeColor: "#101111",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="de" data-theme="dark">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
