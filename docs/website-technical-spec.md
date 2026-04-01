# QCC Plus 官网技术实现规格
## Technical Implementation Specification

**项目代号**: Quantum Gateway
**技术栈**: Next.js 16 + Three.js + GSAP
**目标**: 创建业界领先的3D交互式产品官网

---

## 目录

1. [项目架构](#1-项目架构)
2. [目录结构](#2-目录结构)
3. [核心技术方案](#3-核心技术方案)
4. [组件设计](#4-组件设计)
5. [性能优化](#5-性能优化)
6. [部署方案](#6-部署方案)

---

## 1. 项目架构

### 1.1 技术栈详细清单

```json
{
  "framework": {
    "core": "Next.js 16.0.3",
    "react": "18.2.0",
    "typescript": "5.3.3"
  },
  "3d": {
    "three": "0.160.0",
    "@react-three/fiber": "8.15.0",
    "@react-three/drei": "9.95.0",
    "@react-three/postprocessing": "2.16.0",
    "three-mesh-bvh": "0.7.0"
  },
  "animation": {
    "gsap": "3.12.5",
    "framer-motion": "11.0.3",
    "react-spring": "9.7.3"
  },
  "styling": {
    "tailwindcss": "3.4.1",
    "@emotion/react": "11.11.3",
    "@emotion/styled": "11.11.0"
  },
  "code-editor": {
    "@monaco-editor/react": "4.6.0",
    "monaco-editor": "0.45.0"
  },
  "utils": {
    "clsx": "2.1.0",
    "lodash": "4.17.21",
    "date-fns": "3.3.0"
  },
  "dev": {
    "eslint": "8.56.0",
    "prettier": "3.2.4",
    "@types/three": "0.160.0",
    "autoprefixer": "10.4.17"
  }
}
```

### 1.2 架构模式

```
┌─────────────────────────────────────────┐
│          Next.js App Router             │
│  (App Directory - React Server Components)
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│        Layout & Page Components         │
│  - RootLayout (全局配置)                │
│  - HomePage (主页面)                    │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│         Section Components              │
│  - HeroSection (3D量子隧道)             │
│  - ArchitectureSection (全息架构)       │
│  - FeaturesSection (功能立方体)         │
│  - CodeDemoSection (代码演示)           │
└─────────────────────────────────────────┘
              ↓
┌──────────────────┬──────────────────────┐
│  3D Components   │  UI Components       │
│  - QuantumTunnel │  - Button            │
│  - ParticleField │  - Card              │
│  - HoloCube      │  - CodeEditor        │
└──────────────────┴──────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│          Hooks & Utils                  │
│  - useScrollAnimation                   │
│  - useParticleSystem                    │
│  - usePerformanceMonitor                │
└─────────────────────────────────────────┘
```

---

## 2. 目录结构

```
website/
├── app/                          # Next.js 16 App Router
│   ├── layout.tsx               # 根布局
│   ├── page.tsx                 # 主页
│   ├── globals.css              # 全局样式
│   └── fonts/                   # 字体文件
│       ├── orbitron/
│       └── jetbrains-mono/
│
├── components/                   # 组件目录
│   ├── sections/                # 页面Section组件
│   │   ├── HeroSection/
│   │   │   ├── index.tsx
│   │   │   ├── QuantumTunnel.tsx
│   │   │   ├── ParticleField.tsx
│   │   │   └── styles.module.css
│   │   ├── ArchitectureSection/
│   │   │   ├── index.tsx
│   │   │   ├── HoloArchitecture.tsx
│   │   │   ├── Node3D.tsx
│   │   │   └── DataFlow.tsx
│   │   ├── DataFlowSection/
│   │   │   ├── index.tsx
│   │   │   └── Waterfall.tsx
│   │   ├── FeatureCubeSection/
│   │   │   ├── index.tsx
│   │   │   └── Cube3D.tsx
│   │   ├── CodeDemoSection/
│   │   │   ├── index.tsx
│   │   │   ├── Terminal3D.tsx
│   │   │   └── LiveEditor.tsx
│   │   ├── StatsSection/
│   │   ├── PricingSection/
│   │   └── CTASection/
│   │
│   ├── 3d/                      # 3D组件
│   │   ├── Scene.tsx            # 3D场景容器
│   │   ├── ParticleSystem.tsx   # 粒子系统
│   │   ├── Camera.tsx           # 相机控制
│   │   └── shaders/             # Shader文件
│   │       ├── particle.vert
│   │       ├── particle.frag
│   │       ├── tunnel.vert
│   │       └── tunnel.frag
│   │
│   ├── ui/                      # UI组件
│   │   ├── Button.tsx
│   │   ├── Card.tsx
│   │   ├── GlowText.tsx
│   │   ├── LoadingScreen.tsx
│   │   └── Navigation.tsx
│   │
│   └── animations/              # 动画组件
│       ├── ScrollAnimation.tsx
│       ├── FadeIn.tsx
│       └── CounterAnimation.tsx
│
├── hooks/                       # 自定义Hooks
│   ├── useScrollProgress.ts     # 滚动进度
│   ├── useParticles.ts          # 粒子管理
│   ├── use3DScene.ts            # 3D场景管理
│   ├── usePerformance.ts        # 性能监控
│   ├── useMousePosition.ts      # 鼠标位置
│   └── useMediaQuery.ts         # 响应式检测
│
├── lib/                         # 工具库
│   ├── three-utils.ts           # Three.js工具
│   ├── animation-utils.ts       # 动画工具
│   ├── performance.ts           # 性能优化
│   └── constants.ts             # 常量定义
│
├── public/                      # 静态资源
│   ├── models/                  # 3D模型
│   │   ├── logo.gltf
│   │   └── server-node.gltf
│   ├── textures/                # 纹理
│   │   ├── particle.png
│   │   ├── noise.png
│   │   └── hologram.png
│   ├── images/                  # 图片
│   │   ├── og-image.jpg
│   │   └── favicon.ico
│   └── videos/                  # 视频（备用）
│       └── hero-fallback.mp4
│
├── styles/                      # 样式文件
│   ├── theme.ts                 # 主题配置
│   ├── animations.css           # CSS动画
│   └── utilities.css            # 工具类
│
├── types/                       # TypeScript类型
│   ├── three.d.ts
│   └── components.d.ts
│
├── tailwind.config.ts           # Tailwind配置
├── next.config.js               # Next.js配置
├── tsconfig.json                # TypeScript配置
└── package.json
```

---

## 3. 核心技术方案

### 3.1 量子隧道实现（Hero Section）

#### 3.1.1 粒子系统架构

```typescript
// components/3d/ParticleSystem.tsx
import { useRef, useMemo } from 'react'
import { useFrame } from '@react-three/fiber'
import * as THREE from 'three'

interface ParticleSystemProps {
  count: number        // 粒子数量
  radius: number       // 隧道半径
  speed: number        // 流动速度
  color: THREE.Color   // 粒子颜色
}

export function ParticleSystem({
  count = 50000,
  radius = 5,
  speed = 0.5,
  color = new THREE.Color(0x00d4ff)
}: ParticleSystemProps) {
  const pointsRef = useRef<THREE.Points>(null)

  // 粒子位置和属性
  const particles = useMemo(() => {
    const positions = new Float32Array(count * 3)
    const sizes = new Float32Array(count)
    const colors = new Float32Array(count * 3)

    for (let i = 0; i < count; i++) {
      const i3 = i * 3

      // 圆柱形分布（隧道形状）
      const angle = Math.random() * Math.PI * 2
      const r = radius * (0.7 + Math.random() * 0.3)
      const z = Math.random() * 100 - 50

      positions[i3] = Math.cos(angle) * r
      positions[i3 + 1] = Math.sin(angle) * r
      positions[i3 + 2] = z

      // 粒子大小随机
      sizes[i] = Math.random() * 0.5 + 0.5

      // 颜色渐变（蓝→紫）
      const mixRatio = Math.random()
      colors[i3] = color.r * (1 - mixRatio) + 0.7 * mixRatio
      colors[i3 + 1] = color.g * (1 - mixRatio) + 0 * mixRatio
      colors[i3 + 2] = color.b * (1 - mixRatio) + 1 * mixRatio
    }

    return { positions, sizes, colors }
  }, [count, radius, color])

  // 动画循环
  useFrame((state, delta) => {
    if (!pointsRef.current) return

    const positions = pointsRef.current.geometry.attributes.position.array as Float32Array

    for (let i = 0; i < count; i++) {
      const i3 = i * 3

      // Z轴流动
      positions[i3 + 2] += speed * delta * 10

      // 循环
      if (positions[i3 + 2] > 50) {
        positions[i3 + 2] = -50
      }
    }

    pointsRef.current.geometry.attributes.position.needsUpdate = true

    // 隧道旋转
    pointsRef.current.rotation.z += delta * 0.05
  })

  return (
    <points ref={pointsRef}>
      <bufferGeometry>
        <bufferAttribute
          attach="attributes-position"
          count={count}
          array={particles.positions}
          itemSize={3}
        />
        <bufferAttribute
          attach="attributes-size"
          count={count}
          array={particles.sizes}
          itemSize={1}
        />
        <bufferAttribute
          attach="attributes-color"
          count={count}
          array={particles.colors}
          itemSize={3}
        />
      </bufferGeometry>
      <pointsMaterial
        size={0.05}
        sizeAttenuation
        vertexColors
        transparent
        opacity={0.8}
        blending={THREE.AdditiveBlending}
        depthWrite={false}
      />
    </points>
  )
}
```

#### 3.1.2 自定义Shader增强

```glsl
// components/3d/shaders/particle.vert
attribute float size;
attribute vec3 color;

varying vec3 vColor;

void main() {
  vColor = color;

  vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);

  // 距离相机越远，粒子越小（透视效果）
  gl_PointSize = size * (300.0 / -mvPosition.z);

  gl_Position = projectionMatrix * mvPosition;
}
```

```glsl
// components/3d/shaders/particle.frag
varying vec3 vColor;

void main() {
  // 圆形粒子（不是方形）
  vec2 center = gl_PointCoord - vec2(0.5);
  float dist = length(center);

  if (dist > 0.5) discard;

  // 发光效果
  float glow = 1.0 - smoothstep(0.0, 0.5, dist);

  gl_FragColor = vec4(vColor, glow);
}
```

### 3.2 全息架构图实现

#### 3.2.1 3D节点组件

```typescript
// components/sections/ArchitectureSection/Node3D.tsx
import { useRef, useState } from 'react'
import { useFrame } from '@react-three/fiber'
import { Sphere, Text } from '@react-three/drei'
import * as THREE from 'three'

interface Node3DProps {
  position: [number, number, number]
  label: string
  status: 'healthy' | 'degraded' | 'failed'
  onClick: () => void
}

export function Node3D({ position, label, status, onClick }: Node3DProps) {
  const meshRef = useRef<THREE.Mesh>(null)
  const [hovered, setHovered] = useState(false)

  // 状态颜色映射
  const statusColors = {
    healthy: new THREE.Color(0x00ff88),
    degraded: new THREE.Color(0xffaa00),
    failed: new THREE.Color(0xff0055)
  }

  const color = statusColors[status]

  // 呼吸动画
  useFrame((state) => {
    if (!meshRef.current) return

    const pulse = Math.sin(state.clock.elapsedTime * 2) * 0.1 + 1
    meshRef.current.scale.setScalar(pulse * (hovered ? 1.2 : 1))
  })

  return (
    <group position={position}>
      {/* 球体节点 */}
      <Sphere
        ref={meshRef}
        args={[0.5, 32, 32]}
        onClick={onClick}
        onPointerOver={() => setHovered(true)}
        onPointerOut={() => setHovered(false)}
      >
        <meshStandardMaterial
          color={color}
          emissive={color}
          emissiveIntensity={hovered ? 0.8 : 0.4}
          transparent
          opacity={0.9}
        />
      </Sphere>

      {/* 外层光晕 */}
      <Sphere args={[0.7, 32, 32]}>
        <meshBasicMaterial
          color={color}
          transparent
          opacity={0.2}
          side={THREE.BackSide}
        />
      </Sphere>

      {/* 标签 */}
      <Text
        position={[0, -1, 0]}
        fontSize={0.3}
        color="white"
        anchorX="center"
        anchorY="middle"
      >
        {label}
      </Text>
    </group>
  )
}
```

#### 3.2.2 数据流动线条

```typescript
// components/sections/ArchitectureSection/DataFlow.tsx
import { useRef } from 'react'
import { useFrame } from '@react-three/fiber'
import { Line } from '@react-three/drei'
import * as THREE from 'three'

interface DataFlowProps {
  start: THREE.Vector3
  end: THREE.Vector3
  active: boolean
}

export function DataFlow({ start, end, active }: DataFlowProps) {
  const lineRef = useRef<THREE.Line>(null)
  const particlesRef = useRef<THREE.Points>(null)

  // 粒子沿线条流动
  useFrame((state) => {
    if (!particlesRef.current || !active) return

    const positions = particlesRef.current.geometry.attributes.position.array as Float32Array
    const progress = (Math.sin(state.clock.elapsedTime * 2) + 1) / 2

    for (let i = 0; i < 10; i++) {
      const i3 = i * 3
      const t = (progress + i / 10) % 1

      positions[i3] = THREE.MathUtils.lerp(start.x, end.x, t)
      positions[i3 + 1] = THREE.MathUtils.lerp(start.y, end.y, t)
      positions[i3 + 2] = THREE.MathUtils.lerp(start.z, end.z, t)
    }

    particlesRef.current.geometry.attributes.position.needsUpdate = true
  })

  return (
    <group>
      {/* 连接线 */}
      <Line
        ref={lineRef}
        points={[start, end]}
        color={active ? 0x00d4ff : 0x333333}
        lineWidth={active ? 2 : 1}
        transparent
        opacity={active ? 0.8 : 0.3}
      />

      {/* 流动粒子 */}
      {active && (
        <points ref={particlesRef}>
          <bufferGeometry>
            <bufferAttribute
              attach="attributes-position"
              count={10}
              array={new Float32Array(30)}
              itemSize={3}
            />
          </bufferGeometry>
          <pointsMaterial
            size={0.1}
            color={0x00d4ff}
            transparent
            opacity={0.8}
            blending={THREE.AdditiveBlending}
          />
        </points>
      )}
    </group>
  )
}
```

### 3.3 功能立方体实现

```typescript
// components/sections/FeatureCubeSection/Cube3D.tsx
import { useRef, useState } from 'react'
import { useFrame } from '@react-three/fiber'
import { Box, Text } from '@react-three/drei'
import * as THREE from 'three'

const FEATURES = [
  { face: 'front', title: 'Multi-Tenant', icon: '🔐' },
  { face: 'right', title: 'Smart Routing', icon: '🌐' },
  { face: 'back', title: 'Analytics', icon: '📊' },
  { face: 'left', title: 'Performance', icon: '⚡' },
  { face: 'top', title: 'Security', icon: '🛡️' },
  { face: 'bottom', title: 'Deploy', icon: '🚀' }
]

export function FeatureCube() {
  const cubeRef = useRef<THREE.Mesh>(null)
  const [selectedFace, setSelectedFace] = useState<number | null>(null)
  const [isDragging, setIsDragging] = useState(false)

  // 自动旋转 + 拖拽控制
  useFrame((state, delta) => {
    if (!cubeRef.current || isDragging) return

    cubeRef.current.rotation.x += delta * 0.2
    cubeRef.current.rotation.y += delta * 0.3
  })

  return (
    <group>
      <Box
        ref={cubeRef}
        args={[3, 3, 3]}
        onPointerDown={() => setIsDragging(true)}
        onPointerUp={() => setIsDragging(false)}
      >
        <meshStandardMaterial
          color={0x00d4ff}
          emissive={0x00d4ff}
          emissiveIntensity={0.2}
          transparent
          opacity={0.3}
          wireframe
        />
      </Box>

      {/* 每个面的标签 */}
      {FEATURES.map((feature, index) => {
        const position = getFacePosition(feature.face)
        const rotation = getFaceRotation(feature.face)

        return (
          <group key={index} position={position} rotation={rotation}>
            <Text
              fontSize={0.4}
              color="white"
              anchorX="center"
              anchorY="middle"
            >
              {feature.icon} {feature.title}
            </Text>
          </group>
        )
      })}
    </group>
  )
}

function getFacePosition(face: string): [number, number, number] {
  const offset = 1.6
  const positions: Record<string, [number, number, number]> = {
    front: [0, 0, offset],
    back: [0, 0, -offset],
    right: [offset, 0, 0],
    left: [-offset, 0, 0],
    top: [0, offset, 0],
    bottom: [0, -offset, 0]
  }
  return positions[face]
}

function getFaceRotation(face: string): [number, number, number] {
  const rotations: Record<string, [number, number, number]> = {
    front: [0, 0, 0],
    back: [0, Math.PI, 0],
    right: [0, Math.PI / 2, 0],
    left: [0, -Math.PI / 2, 0],
    top: [-Math.PI / 2, 0, 0],
    bottom: [Math.PI / 2, 0, 0]
  }
  return rotations[face]
}
```

### 3.4 代码演示终端

```typescript
// components/sections/CodeDemoSection/Terminal3D.tsx
import { useState } from 'react'
import { Html } from '@react-three/drei'
import MonacoEditor from '@monaco-editor/react'

export function Terminal3D() {
  const [code, setCode] = useState(`# 启动 QCC Plus
docker-compose up -d

# 测试连接
curl http://localhost:8000/v1/messages \\
  -H "x-api-key: your-key" \\
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`)

  const [output, setOutput] = useState('')

  const runCode = async () => {
    // 模拟执行
    setOutput('✓ Service starting...\n✓ Ready on :8000')
  }

  return (
    <Html
      transform
      distanceFactor={5}
      position={[0, 0, 0]}
      style={{
        width: '800px',
        height: '600px',
        background: 'rgba(10, 10, 15, 0.95)',
        border: '1px solid rgba(0, 212, 255, 0.5)',
        borderRadius: '12px',
        boxShadow: '0 0 50px rgba(0, 212, 255, 0.3)',
        overflow: 'hidden',
        backdropFilter: 'blur(10px)'
      }}
    >
      <div style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        padding: '20px'
      }}>
        {/* 编辑器 */}
        <div style={{ flex: 1, marginBottom: '20px' }}>
          <MonacoEditor
            language="bash"
            theme="vs-dark"
            value={code}
            onChange={(value) => setCode(value || '')}
            options={{
              minimap: { enabled: false },
              fontSize: 14,
              lineNumbers: 'off',
              scrollBeyondLastLine: false,
              fontFamily: 'JetBrains Mono, monospace'
            }}
          />
        </div>

        {/* 运行按钮 */}
        <button
          onClick={runCode}
          style={{
            background: 'linear-gradient(135deg, #00d4ff, #b400ff)',
            border: 'none',
            color: 'white',
            padding: '12px 24px',
            borderRadius: '8px',
            cursor: 'pointer',
            fontSize: '16px',
            fontWeight: 'bold',
            marginBottom: '20px'
          }}
        >
          ▶ Run Code
        </button>

        {/* 输出 */}
        <div style={{
          background: '#000',
          color: '#00ff88',
          padding: '16px',
          borderRadius: '8px',
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: '14px',
          whiteSpace: 'pre-wrap',
          minHeight: '100px'
        }}>
          {output}
        </div>
      </div>
    </Html>
  )
}
```

---

## 4. 组件设计

### 4.1 滚动动画系统

```typescript
// hooks/useScrollAnimation.ts
import { useEffect } from 'react'
import { gsap } from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'

gsap.registerPlugin(ScrollTrigger)

export function useScrollAnimation(
  target: React.RefObject<HTMLElement>,
  animation: gsap.TweenVars,
  triggerOptions?: ScrollTrigger.Vars
) {
  useEffect(() => {
    if (!target.current) return

    const tween = gsap.to(target.current, {
      ...animation,
      scrollTrigger: {
        trigger: target.current,
        start: 'top 80%',
        end: 'bottom 20%',
        toggleActions: 'play none none reverse',
        ...triggerOptions
      }
    })

    return () => {
      tween.kill()
    }
  }, [target, animation, triggerOptions])
}
```

**使用示例**：
```typescript
import { useRef } from 'react'
import { useScrollAnimation } from '@/hooks/useScrollAnimation'

export function MySection() {
  const sectionRef = useRef<HTMLDivElement>(null)

  useScrollAnimation(sectionRef, {
    opacity: 1,
    y: 0,
    duration: 1,
    ease: 'power2.out'
  })

  return (
    <div ref={sectionRef} style={{ opacity: 0, transform: 'translateY(50px)' }}>
      内容...
    </div>
  )
}
```

### 4.2 性能监控Hook

```typescript
// hooks/usePerformance.ts
import { useEffect, useState } from 'react'

interface PerformanceMetrics {
  fps: number
  memory: number
  deviceTier: 'high' | 'medium' | 'low'
}

export function usePerformance(): PerformanceMetrics {
  const [metrics, setMetrics] = useState<PerformanceMetrics>({
    fps: 60,
    memory: 0,
    deviceTier: 'high'
  })

  useEffect(() => {
    let frameCount = 0
    let lastTime = performance.now()
    let animationId: number

    const measureFPS = () => {
      frameCount++
      const currentTime = performance.now()

      if (currentTime >= lastTime + 1000) {
        const fps = Math.round((frameCount * 1000) / (currentTime - lastTime))

        setMetrics(prev => ({
          ...prev,
          fps,
          deviceTier: fps > 50 ? 'high' : fps > 30 ? 'medium' : 'low'
        }))

        frameCount = 0
        lastTime = currentTime
      }

      animationId = requestAnimationFrame(measureFPS)
    }

    animationId = requestAnimationFrame(measureFPS)

    // 内存监控（仅Chrome）
    if ('memory' in performance) {
      const checkMemory = setInterval(() => {
        const mem = (performance as any).memory
        setMetrics(prev => ({
          ...prev,
          memory: Math.round(mem.usedJSHeapSize / 1048576) // MB
        }))
      }, 5000)

      return () => {
        cancelAnimationFrame(animationId)
        clearInterval(checkMemory)
      }
    }

    return () => cancelAnimationFrame(animationId)
  }, [])

  return metrics
}
```

**自动降级示例**：
```typescript
function App() {
  const { deviceTier } = usePerformance()

  const particleCount = {
    high: 50000,
    medium: 20000,
    low: 5000
  }[deviceTier]

  return <ParticleSystem count={particleCount} />
}
```

---

## 5. 性能优化

### 5.1 代码分割策略

```typescript
// app/page.tsx
import dynamic from 'next/dynamic'
import { Suspense } from 'react'

// 动态加载重型3D组件
const HeroSection = dynamic(
  () => import('@/components/sections/HeroSection'),
  {
    ssr: false, // 禁用SSR（Three.js在服务端不可用）
    loading: () => <LoadingScreen />
  }
)

const ArchitectureSection = dynamic(
  () => import('@/components/sections/ArchitectureSection'),
  { ssr: false }
)

export default function HomePage() {
  return (
    <main>
      <Suspense fallback={<LoadingScreen />}>
        <HeroSection />
      </Suspense>

      {/* 视口外的Section懒加载 */}
      <LazyLoad height={1000} offset={500}>
        <ArchitectureSection />
      </LazyLoad>
    </main>
  )
}
```

### 5.2 资源预加载

```typescript
// lib/preload.ts
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader'
import { TextureLoader } from 'three'

export class ResourcePreloader {
  private loader = new GLTFLoader()
  private textureLoader = new TextureLoader()
  private cache = new Map()

  async preloadModels(urls: string[]) {
    const promises = urls.map(url =>
      new Promise((resolve, reject) => {
        if (this.cache.has(url)) {
          resolve(this.cache.get(url))
          return
        }

        this.loader.load(
          url,
          (gltf) => {
            this.cache.set(url, gltf)
            resolve(gltf)
          },
          undefined,
          reject
        )
      })
    )

    return Promise.all(promises)
  }

  async preloadTextures(urls: string[]) {
    const promises = urls.map(url =>
      new Promise((resolve, reject) => {
        if (this.cache.has(url)) {
          resolve(this.cache.get(url))
          return
        }

        this.textureLoader.load(
          url,
          (texture) => {
            this.cache.set(url, texture)
            resolve(texture)
          },
          undefined,
          reject
        )
      })
    )

    return Promise.all(promises)
  }

  getFromCache(url: string) {
    return this.cache.get(url)
  }
}

export const preloader = new ResourcePreloader()
```

### 5.3 Three.js性能优化

```typescript
// lib/three-utils.ts
import * as THREE from 'three'

export function optimizeRenderer(renderer: THREE.WebGLRenderer) {
  // 启用性能优化
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2)) // 限制像素比
  renderer.powerPreference = 'high-performance'
  renderer.antialias = false // 禁用抗锯齿（用后处理替代）

  // 启用物理正确的光照
  renderer.physicallyCorrectLights = true
  renderer.outputEncoding = THREE.sRGBEncoding
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.2

  return renderer
}

export function disposeObject(object: THREE.Object3D) {
  object.traverse((child) => {
    if (child instanceof THREE.Mesh) {
      child.geometry.dispose()

      if (Array.isArray(child.material)) {
        child.material.forEach(material => material.dispose())
      } else {
        child.material.dispose()
      }
    }
  })
}
```

---

## 6. 部署方案

### 6.1 Vercel部署配置

```json
// vercel.json
{
  "framework": "nextjs",
  "buildCommand": "next build",
  "devCommand": "next dev",
  "installCommand": "pnpm install",
  "outputDirectory": ".next",
  "headers": [
    {
      "source": "/models/(.*)",
      "headers": [
        {
          "key": "Cache-Control",
          "value": "public, max-age=31536000, immutable"
        }
      ]
    },
    {
      "source": "/textures/(.*)",
      "headers": [
        {
          "key": "Cache-Control",
          "value": "public, max-age=31536000, immutable"
        }
      ]
    }
  ],
  "redirects": [
    {
      "source": "/admin",
      "destination": "http://localhost:8000/admin",
      "permanent": false
    }
  ]
}
```

### 6.2 Next.js配置优化

```javascript
// next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  swcMinify: true,

  // 图片优化
  images: {
    formats: ['image/avif', 'image/webp'],
    deviceSizes: [640, 750, 828, 1080, 1200, 1920, 2048, 3840],
    imageSizes: [16, 32, 48, 64, 96, 128, 256, 384],
  },

  // 压缩
  compress: true,

  // 实验性功能
  experimental: {
    optimizeCss: true,
    optimizePackageImports: ['three', '@react-three/fiber', '@react-three/drei'],
  },

  // Webpack配置
  webpack: (config, { isServer }) => {
    // 仅客户端打包Three.js
    if (!isServer) {
      config.resolve.fallback = {
        fs: false,
        path: false,
      }
    }

    // GLSL Shader加载器
    config.module.rules.push({
      test: /\.(glsl|vs|fs|vert|frag)$/,
      type: 'asset/source',
    })

    return config
  },
}

module.exports = nextConfig
```

### 6.3 Docker部署（可选）

```dockerfile
# Dockerfile
FROM node:18-alpine AS base

# 依赖安装
FROM base AS deps
RUN apk add --no-cache libc6-compat
WORKDIR /app

COPY package.json pnpm-lock.yaml ./
RUN corepack enable pnpm && pnpm install --frozen-lockfile

# 构建
FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

ENV NEXT_TELEMETRY_DISABLED 1
RUN corepack enable pnpm && pnpm build

# 运行
FROM base AS runner
WORKDIR /app

ENV NODE_ENV production
ENV NEXT_TELEMETRY_DISABLED 1

RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs

EXPOSE 3000

ENV PORT 3000
ENV HOSTNAME "0.0.0.0"

CMD ["node", "server.js"]
```

### 6.4 CI/CD流程

```yaml
# .github/workflows/deploy.yml
name: Deploy Website

on:
  push:
    branches: [main]
    paths:
      - 'website/**'

jobs:
  deploy:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '18'

      - name: Setup pnpm
        uses: pnpm/action-setup@v2
        with:
          version: 8

      - name: Install dependencies
        working-directory: website
        run: pnpm install

      - name: Build
        working-directory: website
        run: pnpm build

      - name: Deploy to Vercel
        uses: amondnet/vercel-action@v25
        with:
          vercel-token: ${{ secrets.VERCEL_TOKEN }}
          vercel-org-id: ${{ secrets.VERCEL_ORG_ID }}
          vercel-project-id: ${{ secrets.VERCEL_PROJECT_ID }}
          working-directory: website
          vercel-args: '--prod'
```

---

## 附录：开发检查清单

### ✅ 开发阶段

- [ ] 初始化Next.js项目
- [ ] 配置TypeScript和ESLint
- [ ] 安装Three.js和依赖
- [ ] 搭建基础Layout
- [ ] 实现粒子系统
- [ ] 实现量子隧道
- [ ] 实现全息架构图
- [ ] 实现功能立方体
- [ ] 实现代码演示终端
- [ ] 添加滚动动画
- [ ] 性能优化
- [ ] 响应式适配
- [ ] SEO配置

### ✅ 测试阶段

- [ ] Chrome测试（最新版）
- [ ] Firefox测试
- [ ] Safari测试
- [ ] Edge测试
- [ ] 移动端Chrome测试
- [ ] 移动端Safari测试
- [ ] 性能测试（Lighthouse）
- [ ] 可访问性测试
- [ ] 跨设备测试

### ✅ 发布阶段

- [ ] 压缩资源
- [ ] 优化图片
- [ ] 配置CDN
- [ ] 配置缓存
- [ ] 配置域名
- [ ] SSL证书
- [ ] 监控配置
- [ ] 备份配置

---

**文档版本**: v1.0
**最后更新**: 2025-11-23
**维护者**: QCC Plus Team
