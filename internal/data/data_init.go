package data

import "context"

func (data *Data) initData() {
	data.entClient.
		Helloworld.
		Create().
		SetName("test").Save(context.Background())
}
