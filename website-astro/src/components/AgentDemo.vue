<template>
  <div class="agent-demo" @click="togglePause">
    <!-- Progress Bar -->
    <div class="progress-bar">
      <div
        v-for="(state, index) in stateSequence"
        :key="index"
        class="progress-step"
        :class="{
          active: index <= currentStateIndex,
          current: index === currentStateIndex
        }"
      >
        <span class="progress-dot" :class="state"></span>
        <span class="progress-label">{{ state }}</span>
      </div>
    </div>

    <!-- Execution Trace -->
    <div class="execution-trace">
      <TransitionGroup name="step">
        <div
          v-for="(step, index) in visibleSteps"
          :key="step.id"
          class="trace-step"
          :class="{ latest: index === visibleSteps.length - 1 }"
        >
          <div class="step-header">
            <span class="step-state" :class="step.state">{{ step.state }}</span>
            <span class="step-action">{{ step.action }}</span>
          </div>

          <div v-if="step.type === 'tool'" class="step-tool">
            <div class="tool-io">
              <div class="io-row">
                <span class="io-label">Input:</span>
                <code class="io-value">{{ formatJson(step.input) }}</code>
              </div>
              <div class="io-row">
                <span class="io-label">Output:</span>
                <code class="io-value" :class="{ success: step.success }">{{ formatJson(step.output) }}</code>
              </div>
            </div>
          </div>

          <div v-else-if="step.type === 'transition'" class="step-description">
            {{ step.description }}
          </div>

          <div v-else-if="step.type === 'result'" class="step-result">
            <span class="result-icon">✓</span>
            {{ step.description }}
          </div>
        </div>
      </TransitionGroup>
    </div>

    <!-- Status Footer -->
    <div class="demo-footer">
      <div class="footer-left">
        <span class="pause-hint">{{ isPaused ? '▶ Click to resume' : '⏸ Click to pause' }}</span>
      </div>
      <div class="footer-right">
        <span class="step-counter">step {{ currentStepIndex + 1 }}/{{ steps.length }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const stateSequence = ['intake', 'explore', 'decide', 'act', 'validate', 'done']

const steps = [
  {
    id: 1,
    state: 'intake',
    type: 'transition',
    action: 'Goal received',
    description: 'Create a hello.txt file with a greeting message'
  },
  {
    id: 2,
    state: 'explore',
    type: 'tool',
    action: 'list_dir',
    input: { path: '.' },
    output: { files: [], count: 0 },
    success: true
  },
  {
    id: 3,
    state: 'decide',
    type: 'transition',
    action: 'Planning',
    description: 'Directory is empty. Will create hello.txt file.'
  },
  {
    id: 4,
    state: 'act',
    type: 'tool',
    action: 'write_file',
    input: { path: 'hello.txt', content: 'Hello, Agent World!' },
    output: { bytes_written: 19, created: true },
    success: true
  },
  {
    id: 5,
    state: 'validate',
    type: 'tool',
    action: 'read_file',
    input: { path: 'hello.txt' },
    output: { content: 'Hello, Agent World!', size: 19 },
    success: true
  },
  {
    id: 6,
    state: 'done',
    type: 'result',
    action: 'Complete',
    description: 'File created and verified successfully'
  }
]

const currentStepIndex = ref(0)
const isPaused = ref(false)
let interval = null

const visibleSteps = computed(() => {
  return steps.slice(0, currentStepIndex.value + 1)
})

const currentStateIndex = computed(() => {
  const currentState = steps[currentStepIndex.value].state
  return stateSequence.indexOf(currentState)
})

const formatJson = (obj) => {
  return JSON.stringify(obj)
}

const togglePause = () => {
  isPaused.value = !isPaused.value
  if (isPaused.value) {
    clearInterval(interval)
  } else {
    startAnimation()
  }
}

const startAnimation = () => {
  interval = setInterval(() => {
    if (currentStepIndex.value < steps.length - 1) {
      currentStepIndex.value++
    } else {
      // Reset after showing final state for a moment
      setTimeout(() => {
        currentStepIndex.value = 0
      }, 1500)
    }
  }, 2000)
}

onMounted(() => {
  startAnimation()
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>

<style scoped>
.agent-demo {
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: 12px;
  overflow: hidden;
  cursor: pointer;
  transition: border-color var(--transition-fast);
}

.agent-demo:hover {
  border-color: var(--border-secondary);
}

/* Progress Bar */
.progress-bar {
  display: flex;
  justify-content: space-between;
  padding: var(--space-md) var(--space-lg);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-primary);
  gap: var(--space-xs);
}

.progress-step {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  opacity: 0.3;
  transition: opacity var(--transition-fast);
}

.progress-step.active {
  opacity: 1;
}

.progress-step.current .progress-dot {
  animation: pulse 1s infinite;
}

.progress-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-muted);
}

.progress-dot.intake { background: #a371f7; }
.progress-dot.explore { background: #00d9ff; }
.progress-dot.decide { background: #ffb000; }
.progress-dot.act { background: #f85149; }
.progress-dot.validate { background: #3fb950; }
.progress-dot.done { background: #3fb950; }

.progress-label {
  font-family: var(--font-mono);
  font-size: 0.65rem;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.progress-step.active .progress-label {
  color: var(--text-secondary);
}

/* Execution Trace */
.execution-trace {
  padding: var(--space-lg);
  min-height: 280px;
  max-height: 320px;
  overflow-y: auto;
}

.trace-step {
  padding: var(--space-md);
  margin-bottom: var(--space-sm);
  background: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: 8px;
  transition: all var(--transition-fast);
}

.trace-step.latest {
  border-color: var(--accent-cyan);
  box-shadow: 0 0 0 1px rgba(0, 217, 255, 0.1);
}

.step-header {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-bottom: var(--space-xs);
}

.step-state {
  font-family: var(--font-mono);
  font-size: 0.7rem;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.step-state.intake { background: rgba(163, 113, 247, 0.2); color: #a371f7; }
.step-state.explore { background: rgba(0, 217, 255, 0.2); color: #00d9ff; }
.step-state.decide { background: rgba(255, 176, 0, 0.2); color: #ffb000; }
.step-state.act { background: rgba(248, 81, 73, 0.2); color: #f85149; }
.step-state.validate { background: rgba(63, 185, 80, 0.2); color: #3fb950; }
.step-state.done { background: rgba(63, 185, 80, 0.2); color: #3fb950; }

.step-action {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-primary);
}

.step-description {
  font-size: 0.8rem;
  color: var(--text-secondary);
  line-height: 1.5;
}

.step-tool {
  margin-top: var(--space-xs);
}

.tool-io {
  font-family: var(--font-mono);
  font-size: 0.75rem;
}

.io-row {
  display: flex;
  gap: var(--space-sm);
  padding: var(--space-xs) 0;
}

.io-label {
  color: var(--text-muted);
  min-width: 50px;
}

.io-value {
  color: var(--text-secondary);
  background: var(--bg-tertiary);
  padding: 2px 6px;
  border-radius: 4px;
  word-break: break-all;
}

.io-value.success {
  color: var(--accent-green);
}

.step-result {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-size: 0.85rem;
  color: var(--accent-green);
  font-weight: 500;
}

.result-icon {
  font-size: 1rem;
}

/* Footer */
.demo-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-sm) var(--space-lg);
  background: var(--bg-tertiary);
  border-top: 1px solid var(--border-primary);
  font-family: var(--font-mono);
  font-size: 0.7rem;
}

.pause-hint {
  color: var(--text-muted);
}

.step-counter {
  color: var(--text-secondary);
}

/* Transition animations */
.step-enter-active {
  transition: all 0.4s ease-out;
}

.step-enter-from {
  opacity: 0;
  transform: translateY(-10px);
}

.step-leave-active {
  transition: all 0.2s ease-in;
}

.step-leave-to {
  opacity: 0;
}

@keyframes pulse {
  0%, 100% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.8; }
}

/* Responsive */
@media (max-width: 640px) {
  .progress-label {
    display: none;
  }

  .progress-bar {
    justify-content: center;
    gap: var(--space-md);
  }

  .execution-trace {
    min-height: 240px;
  }
}
</style>
