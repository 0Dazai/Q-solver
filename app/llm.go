package app

import "context"

func (a *App) TestConnection(apiKey, baseURL, provider, model string) string {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.llmService.TestConnection(ctx, apiKey, baseURL, provider, model)
}

func (a *App) GetModels(apiKey, baseURL, provider string) ([]string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.llmService.GetModels(ctx, apiKey, baseURL, provider)
}
