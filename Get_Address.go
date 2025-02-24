package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type WalletItem struct {
	Result struct {
		Data struct {
			Json []struct {
				Address string   `json:"address"`
				Labels  []string `json:"labels"`
			} `json:"json"`
		} `json:"data"`
	} `json:"result"`
}

type WalletOutput struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

func fetchWalletData(channelID string) ([]WalletItem, error) {
	url := fmt.Sprintf("https://chain.fm/api/trpc/walletItem.listBuyChannel?batch=1&input={\"0\":{\"json\":{\"chanelId\":\"%s\"}}}", channelID)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求API失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	var data []WalletItem
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("解析JSON失败: %v", err)
	}

	return data, nil
}

func main() {
	// 检查是否有参数
	if len(os.Args) < 2 {
		fmt.Println("使用方法: go run test.go <channelID1> [channelID2] [channelID3] ...")
		return
	}

	// 获取所有channelID参数（跳过第一个参数，因为是程序名）
	channelIDs := os.Args[1:]

	// 处理每个channelID
	for _, channelID := range channelIDs {
		fmt.Printf("正在处理 channelID: %s\n", channelID)
		
		// 获取数据
		data, err := fetchWalletData(channelID)
		if err != nil {
			fmt.Printf("获取数据失败 (channelID: %s): %v\n", channelID, err)
			continue // 继续处理下一个channelID
		}

		// 处理所有钱包数据
		var outputs []WalletOutput
		for _, item := range data[0].Result.Data.Json {
			if len(item.Labels) == 0 {
				continue
			}

			output := WalletOutput{
				Address: item.Address,
				Label:   item.Labels[0],
			}
			outputs = append(outputs, output)
		}

		// 转换为JSON
		jsonData, err := json.MarshalIndent(outputs, "", "  ")
		if err != nil {
			fmt.Printf("转换JSON失败 (channelID: %s): %v\n", channelID, err)
			continue
		}

		// 写入文件
		filename := fmt.Sprintf("%s.json", channelID)
		if err := ioutil.WriteFile(filename, jsonData, 0644); err != nil {
			fmt.Printf("写入文件失败 %s: %v\n", filename, err)
			continue
		}

		fmt.Printf("成功保存文件: %s\n", filename)
	}
}
