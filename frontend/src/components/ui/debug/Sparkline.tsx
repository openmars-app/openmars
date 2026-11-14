import React, { useRef, useEffect } from "react";

interface SparklineProps {
  data: (number | null)[];
  width?: number;
  height?: number;
  color: string;
  fillColor?: string;
  min?: number;
  max?: number;
  label?: string;
  currentValue?: string;
}

const Sparkline: React.FC<SparklineProps> = ({
  data,
  width = 200,
  height = 40,
  color,
  fillColor,
  min: fixedMin,
  max: fixedMax,
  label,
  currentValue,
}) => {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    ctx.scale(dpr, dpr);

    ctx.clearRect(0, 0, width, height);

    const values = data.filter(
      (value): value is number => value !== null && Number.isFinite(value),
    );
    const dataMin = fixedMin ?? Math.min(0, ...values);
    const dataMax = fixedMax ?? Math.max(1, ...values);
    const range = dataMax - dataMin || 1;

    const padding = { top: label ? 14 : 2, bottom: 2, left: 1, right: 1 };
    const plotW = width - padding.left - padding.right;
    const plotH = height - padding.top - padding.bottom;

    const toX = (i: number) => padding.left + (i / Math.max(1, data.length - 1)) * plotW;
    const toY = (v: number) => padding.top + plotH - ((v - dataMin) / range) * plotH;

    // Missing/paused samples are gaps, never fabricated zero measurements.
    let segment: { x: number; y: number }[] = [];
    const drawSegment = () => {
      if (segment.length < 2) {
        segment = [];
        return;
      }
      if (fillColor) {
        ctx.beginPath();
        ctx.moveTo(segment[0].x, padding.top + plotH);
        for (const point of segment) ctx.lineTo(point.x, point.y);
        ctx.lineTo(segment.at(-1)!.x, padding.top + plotH);
        ctx.closePath();
        ctx.fillStyle = fillColor;
        ctx.fill();
      }
      ctx.beginPath();
      ctx.moveTo(segment[0].x, segment[0].y);
      for (const point of segment.slice(1)) ctx.lineTo(point.x, point.y);
      ctx.strokeStyle = color;
      ctx.lineWidth = 1.5;
      ctx.stroke();
      segment = [];
    };
    for (const [index, value] of data.entries()) {
      if (value === null || !Number.isFinite(value)) drawSegment();
      else segment.push({ x: toX(index), y: toY(value) });
    }
    drawSegment();

    // Label text
    if (label || currentValue) {
      ctx.font = "11px sans-serif";
      if (label) {
        ctx.fillStyle = "rgba(255,255,255,0.5)";
        ctx.textAlign = "left";
        ctx.fillText(label, 2, 10);
      }
      if (currentValue) {
        ctx.font = "11px ui-monospace, SFMono-Regular, Menlo, Consolas, monospace";
        ctx.fillStyle = color;
        ctx.textAlign = "right";
        ctx.fillText(currentValue, width - 2, 10);
      }
    }
  }, [data, width, height, color, fillColor, fixedMin, fixedMax, label, currentValue]);

  return (
    <canvas
      ref={canvasRef}
      style={{
        width: `${width}px`,
        maxWidth: "100%",
        height: `${height}px`,
        display: "block",
      }}
    />
  );
};

export default Sparkline;
