import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";

const instrumentSans = localFont({
  src: "../../public/fonts/InstrumentSans-VariableFont_wdth,wght.ttf",
  variable: "--font-instrument-sans",
});

const instrumentSerif = localFont({
  src: "../../public/fonts/InstrumentSerif-Regular.ttf",
  variable: "--font-instrument-serif",
  weight: "400",
});

export const metadata: Metadata = {
  title: "Reso | Private watch rooms",
  description: "Temporary private rooms for sharing a screen and watching together.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`${instrumentSans.variable} ${instrumentSerif.variable}`}
    >
      <body>{children}</body>
    </html>
  );
}
