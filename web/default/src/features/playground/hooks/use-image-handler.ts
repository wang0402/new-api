import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import {
  getImageGenerationTask,
  submitImageGenerationTask,
} from '../api'
import { ERROR_MESSAGES, MESSAGE_STATUS } from '../constants'
import {
  finalizeMessage,
  updateAssistantMessageWithError,
  updateLastAssistantMessage,
} from '../lib'
import type {
  ImageGenerationResponse,
  ImageGenerationTask,
  Message,
  PlaygroundConfig,
} from '../types'

interface UseImageHandlerOptions {
  config: PlaygroundConfig
  onMessageUpdate: (updater: (prev: Message[]) => Message[]) => void
}

const IMAGE_TASK_POLL_INTERVAL_MS = 2000
const IMAGE_TASK_TIMEOUT_MS = 10 * 60 * 1000

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function extractTaskErrorMessage(error: unknown): string {
  const err = error as {
    error?: { message?: string; code?: string }
    message?: string
  }
  return (
    err?.error?.message ||
    err?.message ||
    ERROR_MESSAGES.API_REQUEST_ERROR
  )
}

async function waitForImageTask(taskId: string): Promise<ImageGenerationResponse> {
  const startedAt = Date.now()

  while (Date.now() - startedAt < IMAGE_TASK_TIMEOUT_MS) {
    await sleep(IMAGE_TASK_POLL_INTERVAL_MS)

    const task: ImageGenerationTask = await getImageGenerationTask(taskId)
    if (task.status === 'succeeded') {
      return task.response || { created: 0, data: [] }
    }
    if (task.status === 'failed') {
      throw new Error(extractTaskErrorMessage(task.error))
    }
  }

  throw new Error('Image generation task timed out')
}

export function useImageHandler({
  config,
  onMessageUpdate,
}: UseImageHandlerOptions) {
  const [isGenerating, setIsGenerating] = useState(false)

  const sendImage = useCallback(
    async (prompt: string) => {
      setIsGenerating(true)

      try {
        const task = await submitImageGenerationTask({
          model: config.model,
          group: config.group,
          prompt,
          size: config.image_size,
          quality: config.image_quality,
          n: config.image_n,
        })
        const response = await waitForImageTask(task.id)

        const images = response.data || []
        const content =
          images[0]?.revised_prompt ||
          (images.length > 0 ? 'Image generated' : 'No image returned')

        onMessageUpdate((prev) =>
          updateLastAssistantMessage(prev, (message) => ({
            ...finalizeMessage({
              ...message,
              versions: [
                {
                  ...message.versions[0],
                  content,
                },
              ],
            }),
            images,
            status: MESSAGE_STATUS.COMPLETE,
          }))
        )
      } catch (error: unknown) {
        const err = error as {
          response?: {
            data?: {
              message?: string
              error?: { message?: string; code?: string }
            }
          }
          message?: string
        }
        const message =
          err?.response?.data?.error?.message ||
          err?.response?.data?.message ||
          err?.message ||
          ERROR_MESSAGES.API_REQUEST_ERROR

        toast.error(message)
        onMessageUpdate((prev) =>
          updateAssistantMessageWithError(
            prev,
            message,
            err?.response?.data?.error?.code || undefined
          )
        )
      } finally {
        setIsGenerating(false)
      }
    },
    [
      config.group,
      config.image_n,
      config.image_quality,
      config.image_size,
      config.model,
      onMessageUpdate,
    ]
  )

  return {
    sendImage,
    isGenerating,
  }
}
