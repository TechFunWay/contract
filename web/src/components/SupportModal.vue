<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition duration-200 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-150 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <!-- 主弹窗：赞赏说明 + 收款码。外层可滚动：内容高于小屏（尤其手机）
           时弹窗内部滚动；点击遮罩或右上角关闭。
           注意：卡片带 backdrop-blur，会成为内部 fixed 元素的包含块，
           因此金额弹窗必须放在卡片之外（见下方独立浮层）。 -->
      <div v-if="support.show" class="fixed inset-0 z-[70] overflow-y-auto overscroll-contain bg-black/50 backdrop-blur-sm" @click="support.dismiss()">
        <div class="flex min-h-full items-center justify-center p-4">
          <div class="relative surface rounded-2xl shadow-card w-full max-w-sm text-center overflow-hidden" @click.stop>
            <button
              class="absolute top-3 right-3 z-10 p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
              aria-label="关闭"
              @click="support.dismiss()"
            >
              <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>

            <div class="px-5 pt-6 pb-5 sm:px-6 sm:pt-7 sm:pb-6">
              <div class="w-14 h-14 mx-auto mb-3 rounded-2xl bg-brand-gradient flex items-center justify-center shadow-glow">
                <span class="text-2xl">☕</span>
              </div>
              <h3 class="text-lg font-bold text-foreground mb-2">请作者喝杯咖啡</h3>

              <p class="text-sm text-muted-foreground leading-relaxed mb-1">
                这个应用免费、无广告，数据完全保存在你自己的设备上。
              </p>
              <p class="text-sm text-muted-foreground leading-relaxed mb-4">
                如果它帮到了你，欢迎请作者喝杯咖啡——<strong class="text-foreground">金额随意，1 元也是心意</strong>。
                <br />
                <span class="text-xs">不赞赏也完全没有问题，<strong>不支付不影响任何功能</strong>。</span>
              </p>

              <div class="flex justify-center my-4">
                <div>
                  <!-- 明确宽高：加载前先占位避免高度跳动；max-h 限制矮屏下被撑出屏幕 -->
                  <img
                    :src="donateQr"
                    alt="微信赞赏码"
                    width="352"
                    height="480"
                    decoding="async"
                    class="mx-auto w-[190px] h-auto max-h-[32vh] max-w-full object-contain rounded-xl border border-border bg-white p-1 shadow-card"
                  />
                  <span class="block mt-2 text-xs text-muted-foreground">微信扫码赞赏</span>
                </div>
              </div>

              <p v-if="support.errorText" class="text-xs text-destructive mt-3">{{ support.errorText }}</p>

              <div class="flex justify-center gap-3 mt-5">
                <button
                  class="flex-1 px-4 py-2.5 rounded-xl text-sm font-semibold text-muted-foreground hover:bg-muted transition-colors"
                  :disabled="support.sending"
                  @click="support.dismiss()"
                >
                  暂不支持
                </button>
                <button
                  class="flex-1 px-4 py-2.5 rounded-xl text-sm font-semibold bg-brand-gradient text-white shadow-glow hover:opacity-90 transition-opacity disabled:opacity-50 flex items-center justify-center gap-1.5"
                  :disabled="support.sending"
                  @click="openAmount"
                >
                  <svg v-if="!support.sending" class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>
                  {{ support.sending ? '发送中…' : '已支持' }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 金额输入弹窗（点击「已支持」后出现）。
         1) 独立于主弹窗、直接挂到 body：卡片上的 backdrop-blur 会让内部
            fixed 元素相对卡片定位，嵌在卡片里会导致弹窗错位；
         2) 不使用 Transition：淡入依赖 requestAnimationFrame，若 webview
            节流帧调度，弹窗会停在 opacity-0 而看不见。 -->
    <div v-if="amountDialog" class="fixed inset-0 z-[80] overflow-y-auto overscroll-contain bg-black/50 backdrop-blur-sm" @click="closeAmount">
      <div class="flex min-h-full items-center justify-center p-4">
        <div class="relative surface rounded-2xl shadow-card w-full max-w-xs p-5 text-center" @click.stop>
          <h4 class="text-base font-bold text-foreground">填写赞赏金额</h4>
          <p class="mt-1 text-xs text-muted-foreground">金额随意，1 元也是心意；仅用于接收端统计，不做支付核验</p>
          <div class="mt-3 flex flex-wrap justify-center gap-2">
            <button
              v-for="p in presets" :key="p"
              class="rounded-full border border-border bg-muted/40 px-3 py-1 text-sm font-medium text-foreground transition-colors hover:bg-muted"
              :class="{ '!border-brand-500 !bg-brand-500/10 !text-brand-600 dark:!text-brand-300': amount === p }"
              @click="selectPreset(p)"
            >
              {{ p }} 元
            </button>
          </div>
          <div class="mt-3 flex items-center justify-center gap-1.5">
            <span class="text-sm text-muted-foreground">或输入</span>
            <input
              v-model.number="customAmount"
              type="number"
              min="0"
              step="0.01"
              placeholder="金额（元）"
              class="input-field w-28 text-center"
              @input="amount = Number(customAmount) || 0"
            />
          </div>
          <p v-if="amount === 0" class="mt-2 text-xs text-amber-500">未填写金额将以 0 元上报</p>
          <p v-if="support.errorText" class="mt-2 text-xs text-destructive">{{ support.errorText }}</p>
          <div class="mt-4 flex gap-3">
            <button class="btn-ghost flex-1" :disabled="support.sending" @click="closeAmount">返回</button>
            <button class="btn-brand flex-1" :disabled="support.sending" @click="confirmAmount">
              {{ support.sending ? '发送中…' : '确定' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useSupportStore } from '../stores/support'
import donateQr from '../assets/donate-wechat.png'

const support = useSupportStore()

// 金额输入弹窗状态
const amountDialog = ref(false)
const presets = [1, 5, 10, 50]
const customAmount = ref<number | ''>('')
const amount = ref<number>(0)

// 主弹窗关闭时（含关闭按钮/遮罩/发送成功）一并复位金额弹窗
watch(
  () => support.show,
  (visible) => {
    if (!visible) resetAmount()
  },
)

function resetAmount() {
  amountDialog.value = false
  customAmount.value = ''
  amount.value = 0
}

function openAmount() {
  resetAmount()
  support.errorText = ''
  amountDialog.value = true
}

function selectPreset(p: number) {
  amount.value = p
  customAmount.value = ''
}

function closeAmount() {
  if (support.sending) return
  resetAmount()
}

function confirmAmount() {
  if (support.sending) return
  void support.confirmSupported(amount.value || 0)
}
</script>
