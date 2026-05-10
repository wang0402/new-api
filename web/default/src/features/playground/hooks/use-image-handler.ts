import { useCallback, useState } from 'react'
import { toast } from 'sonner'
import { sendImageGeneration } from '../api'
import { ERROR_MESSAGES, MESSAGE_STATUS } from '../constants'
import {
  finalizeMessage,
  updateAssistantMessageWithError,
  updateLastAssistantMessage,
} from '../lib'
import type { Message, PlaygroundConfig } from '../types'

interface UseImageHandlerOptions {
  config: PlaygroundConfig
  onMessageUpdate: (updater: (prev: Message[]) => Message[]) => void
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
        const response = await sendImageGeneration({
          model: config.model,
          group: config.group,
          prompt,
          size: config.image_size,
          quality: config.image_quality,
          n: config.image_n,
        })

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
