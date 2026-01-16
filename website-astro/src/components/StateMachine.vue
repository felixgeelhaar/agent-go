<template>
  <div class="state-machine">
    <svg viewBox="0 0 400 400" class="state-diagram">
      <!-- Grid pattern for technical feel -->
      <defs>
        <pattern id="grid" width="20" height="20" patternUnits="userSpaceOnUse">
          <path d="M 20 0 L 0 0 0 20" fill="none" stroke="#21262d" stroke-width="0.5"/>
        </pattern>
        <filter id="glow">
          <feGaussianBlur stdDeviation="3" result="coloredBlur"/>
          <feMerge>
            <feMergeNode in="coloredBlur"/>
            <feMergeNode in="SourceGraphic"/>
          </feMerge>
        </filter>
      </defs>

      <rect width="100%" height="100%" fill="url(#grid)" opacity="0.3"/>

      <!-- Connection lines -->
      <g class="connections">
        <!-- intake -> explore -->
        <line x1="140" y1="80" x2="200" y2="140" stroke="#30363d" stroke-width="2" />
        <!-- explore -> decide -->
        <line x1="260" y1="140" x2="320" y2="200" stroke="#30363d" stroke-width="2" />
        <!-- decide -> act -->
        <line x1="320" y1="260" x2="260" y2="320" stroke="#30363d" stroke-width="2" />
        <!-- act -> validate -->
        <line x1="200" y1="320" x2="140" y2="260" stroke="#30363d" stroke-width="2" />
        <!-- validate -> explore (loop) -->
        <path d="M 80 200 Q 40 200 40 170 Q 40 140 80 140 L 140 140"
              fill="none" stroke="#30363d" stroke-width="2" stroke-dasharray="4,4" />
        <!-- to done -->
        <line x1="200" y1="200" x2="200" y2="340" stroke="#30363d" stroke-width="1" stroke-dasharray="4,4" />
        <!-- to failed -->
        <line x1="200" y1="200" x2="200" y2="60" stroke="#30363d" stroke-width="1" stroke-dasharray="4,4" />
      </g>

      <!-- State nodes -->
      <g class="states">
        <!-- Intake -->
        <g class="state-node" :class="{ active: activeState === 'intake' }">
          <circle cx="100" cy="80" r="30" class="state-circle state-intake" />
          <text x="100" y="85" text-anchor="middle" class="state-label">intake</text>
        </g>

        <!-- Explore -->
        <g class="state-node" :class="{ active: activeState === 'explore' }">
          <circle cx="200" cy="140" r="35" class="state-circle state-explore" />
          <text x="200" y="145" text-anchor="middle" class="state-label">explore</text>
        </g>

        <!-- Decide -->
        <g class="state-node" :class="{ active: activeState === 'decide' }">
          <circle cx="320" cy="200" r="30" class="state-circle state-decide" />
          <text x="320" y="205" text-anchor="middle" class="state-label">decide</text>
        </g>

        <!-- Act (highlighted as side-effect state) -->
        <g class="state-node" :class="{ active: activeState === 'act' }">
          <circle cx="260" cy="320" r="35" class="state-circle state-act" />
          <text x="260" y="325" text-anchor="middle" class="state-label">act</text>
          <text x="260" y="345" text-anchor="middle" class="state-sublabel">side effects</text>
        </g>

        <!-- Validate -->
        <g class="state-node" :class="{ active: activeState === 'validate' }">
          <circle cx="80" cy="200" r="30" class="state-circle state-validate" />
          <text x="80" y="205" text-anchor="middle" class="state-label">validate</text>
        </g>

        <!-- Done (terminal) -->
        <g class="state-node" :class="{ active: activeState === 'done' }">
          <rect x="170" y="360" width="60" height="30" rx="4" class="state-rect state-done" />
          <text x="200" y="380" text-anchor="middle" class="state-label terminal">done</text>
        </g>

        <!-- Failed (terminal) -->
        <g class="state-node" :class="{ active: activeState === 'failed' }">
          <rect x="170" y="20" width="60" height="30" rx="4" class="state-rect state-failed" />
          <text x="200" y="40" text-anchor="middle" class="state-label terminal">failed</text>
        </g>
      </g>

      <!-- Animated dot following the path -->
      <circle
        v-if="showDot"
        :cx="dotPosition.x"
        :cy="dotPosition.y"
        r="6"
        class="animated-dot"
        filter="url(#glow)"
      />
    </svg>

    <!-- Status indicator -->
    <div class="status-bar">
      <div class="status-item">
        <span class="status-dot" :class="activeState"></span>
        <span class="status-label">current: <code>{{ activeState }}</code></span>
      </div>
      <div class="status-item">
        <span class="status-text">step {{ currentStep + 1 }}/{{ steps.length }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'

const states = ['intake', 'explore', 'decide', 'act', 'validate', 'done']
const steps = [
  { state: 'intake', x: 100, y: 80 },
  { state: 'explore', x: 200, y: 140 },
  { state: 'decide', x: 320, y: 200 },
  { state: 'act', x: 260, y: 320 },
  { state: 'validate', x: 80, y: 200 },
  { state: 'explore', x: 200, y: 140 },
  { state: 'decide', x: 320, y: 200 },
  { state: 'done', x: 200, y: 375 },
]

const currentStep = ref(0)
const showDot = ref(true)

const activeState = computed(() => steps[currentStep.value].state)
const dotPosition = computed(() => ({
  x: steps[currentStep.value].x,
  y: steps[currentStep.value].y
}))

let interval = null

onMounted(() => {
  interval = setInterval(() => {
    currentStep.value = (currentStep.value + 1) % steps.length
  }, 1500)
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>

<style scoped>
.state-machine {
  width: 100%;
  max-width: 420px;
  position: relative;
}

.state-diagram {
  width: 100%;
  height: auto;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: 12px;
}

.state-circle {
  fill: var(--bg-tertiary);
  stroke: var(--border-primary);
  stroke-width: 2;
  transition: all 0.3s ease;
}

.state-rect {
  fill: var(--bg-tertiary);
  stroke: var(--border-primary);
  stroke-width: 2;
  transition: all 0.3s ease;
}

.state-node.active .state-circle,
.state-node.active .state-rect {
  filter: url(#glow);
}

.state-intake { stroke: #a371f7; }
.state-explore { stroke: #00d9ff; }
.state-decide { stroke: #ffb000; }
.state-act { stroke: #f85149; fill: rgba(248, 81, 73, 0.1); }
.state-validate { stroke: #3fb950; }
.state-done { stroke: #3fb950; }
.state-failed { stroke: #f85149; }

.state-node.active .state-intake { stroke: #a371f7; fill: rgba(163, 113, 247, 0.2); }
.state-node.active .state-explore { stroke: #00d9ff; fill: rgba(0, 217, 255, 0.2); }
.state-node.active .state-decide { stroke: #ffb000; fill: rgba(255, 176, 0, 0.2); }
.state-node.active .state-act { stroke: #f85149; fill: rgba(248, 81, 73, 0.3); }
.state-node.active .state-validate { stroke: #3fb950; fill: rgba(63, 185, 80, 0.2); }
.state-node.active .state-done { stroke: #3fb950; fill: rgba(63, 185, 80, 0.2); }
.state-node.active .state-failed { stroke: #f85149; fill: rgba(248, 81, 73, 0.2); }

.state-label {
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 600;
  fill: var(--text-secondary);
  transition: fill 0.3s ease;
}

.state-label.terminal {
  font-size: 9px;
}

.state-sublabel {
  font-family: var(--font-mono);
  font-size: 7px;
  fill: var(--accent-red);
  opacity: 0.8;
}

.state-node.active .state-label {
  fill: var(--text-primary);
}

.animated-dot {
  fill: var(--accent-cyan);
  transition: cx 0.5s ease, cy 0.5s ease;
}

.status-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: var(--space-md);
  padding: var(--space-sm) var(--space-md);
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: 8px;
  font-size: 0.75rem;
  font-family: var(--font-mono);
}

.status-item {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-muted);
  animation: pulse 2s infinite;
}

.status-dot.intake { background: #a371f7; }
.status-dot.explore { background: #00d9ff; }
.status-dot.decide { background: #ffb000; }
.status-dot.act { background: #f85149; }
.status-dot.validate { background: #3fb950; }
.status-dot.done { background: #3fb950; }
.status-dot.failed { background: #f85149; }

.status-label {
  color: var(--text-secondary);
}

.status-label code {
  color: var(--accent-cyan);
  background: var(--bg-secondary);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 0.7rem;
}

.status-text {
  color: var(--text-muted);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}
</style>
