package redis

import (
	"context"
	"time"
)

const jwtPrefix = "jwt."

func getJWTKey(token string) string {
	return servicePrefix + jwtPrefix + token
}

func (c *Client) WriteJWTToBlackList(ctx context.Context, jwtStr string, jwtTTL time.Duration) error {
	return c.client.Set(ctx, getJWTKey(jwtStr), true, jwtTTL).Err()
}

func (c *Client) CheckJWTInBlackList(ctx context.Context, jwtStr string) error {
	//log.Println("\nCheckJWTInBlackList:   ", c.client.Get(ctx, getJWTKey(jwtStr)))
	//keys, err := c.client.Keys(ctx, "*").Result()
	//if err != nil {
	//	log.Fatal(err)
	//}
	//for _, key := range keys {
	//	log.Println(key)
	//}
	//log.Println(c.client.ClientList(ctx), "\n\n", c.client.Options())
	return c.client.Get(ctx, getJWTKey(jwtStr)).Err()
}
