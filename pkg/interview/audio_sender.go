package interview

import "context"

func runAudioSender(
	ctx context.Context,
	packets <-chan []byte,
	current func() ASRClient,
	onError func(error),
) {
	for {
		select {
		case <-ctx.Done():
			return
		case packet, ok := <-packets:
			if !ok {
				return
			}
			client := current()
			if client == nil {
				continue
			}
			if err := client.SendAudio(packet); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}
