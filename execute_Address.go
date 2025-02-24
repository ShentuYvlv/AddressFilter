package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// AddrData 存储地址数据
type AddrData struct {
	PageProps struct {
		AddressInfo struct {
			TotalProfit interface{} `json:"total_profit"`
			SolBalance  interface{} `json:"sol_balance"`
			WinRate     interface{} `json:"winrate"`
			TwitterName string      `json:"twitter_name"`
		} `json:"addressInfo"`
	} `json:"pageProps"`
}

// Result 存储结果
type Result struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

// AddressItem 存储地址项
type AddressItem struct {
	Address string `json:"address"`
	Label   string `json:"label"`
}

// FetchAndAnalyzeData 获取并分析数据
func FetchAndAnalyzeData(address string) (*Result, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ProxyServer("socks5://127.0.0.1:10808"),
		chromedp.Flag("headless", true),
		chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36`),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var jsonContent string
	apiURL := fmt.Sprintf("https://gmgn.ai/_next/data/uFrHZZO4a9NWehviXLbes/sol/address/%s.json?chain=sol", address)
	
	err := chromedp.Run(ctx,
		chromedp.Navigate(apiURL),
		chromedp.Text("body", &jsonContent),
	)

	if err != nil {
		return nil, fmt.Errorf("访问失败: %v", err)
	}

	var data AddrData
	if err := json.Unmarshal([]byte(jsonContent), &data); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v", err)
	}

	addr := data.PageProps.AddressInfo
	
	// 转换数值
	var totalProfit, solBalance, winRate float64
	
	switch v := addr.TotalProfit.(type) {
	case float64:
		totalProfit = v
	case string:
		fmt.Sscanf(v, "%f", &totalProfit)
	}
	
	switch v := addr.SolBalance.(type) {
	case float64:
		solBalance = v
	case string:
		fmt.Sscanf(v, "%f", &solBalance)
	}
	
	switch v := addr.WinRate.(type) {
	case float64:
		winRate = v
	case string:
		fmt.Sscanf(v, "%f", &winRate)
	}

	if (totalProfit >= 1000000 && solBalance >= 20 && winRate >= 0.1) ||
		(totalProfit >= 10000 && winRate >= 0.755) {
		log.Printf("地址 %s 符合条件 totalProfit:%.2f,solBalance:%.2f,winRate:%.3f,name:%s", 
			address, totalProfit, solBalance, winRate, addr.TwitterName)
		return &Result{
			Address: address,
			Label:   fmt.Sprintf("profit:%.2f,balance:%.2f,winrate:%.3f,name:%s", 
				totalProfit, solBalance, winRate, addr.TwitterName),
		}, nil
	}else{
		log.Printf("地址 %s 不符合条件 totalProfit:%.2f,solBalance:%.2f,winRate:%.3f,name:%s", 
			address, totalProfit, solBalance, winRate, addr.TwitterName)
	}

	return nil, nil
}

func readAddressesFromFile(filePath string) ([]AddressItem, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %v", err)
	}

	var addresses []AddressItem
	ext := filepath.Ext(filePath)

	switch ext {
	case ".json":
		// 处理JSON文件
		if err := json.Unmarshal(content, &addresses); err != nil {
			return nil, fmt.Errorf("解析JSON失败: %v", err)
		}
	case ".txt":
		// 处理TXT文件
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		// 跳过标题行（如果存在）
		if scanner.Scan() {
			firstLine := scanner.Text()
			if !strings.Contains(firstLine, "  ") {
				// 如果第一行不是标题行，回到开始处
				scanner = bufio.NewScanner(strings.NewReader(string(content)))
			}
		}

		// 处理每一行
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				address := parts[0]
				label := strings.Join(parts[1:], " ")
				addresses = append(addresses, AddressItem{
					Address: address,
					Label:   label,
				})
			}
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("读取TXT文件失败: %v", err)
		}
	default:
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	return addresses, nil
}

// 去重和过滤结果
func deduplicateAndFilter(results []Result) []Result {
	// 使用map来去重和过滤
	uniqueAddresses := make(map[string]Result)
	for _, result := range results {
		// 解析 winrate
		var winrate float64
		if _, err := fmt.Sscanf(strings.Split(result.Label, "winrate:")[1], "%f", &winrate); err != nil {
			log.Printf("解析winrate失败 for %s: %v", result.Address, err)
			continue
		}

		// 如果 winrate <= 0.1，跳过这个地址
		if winrate <= 0.1 {
			continue
		}

		// 如果这个地址已经存在，检查是否需要更新
		if existing, exists := uniqueAddresses[result.Address]; exists {
			// 获取现有记录的 winrate
			var existingWinrate float64
			if _, err := fmt.Sscanf(strings.Split(existing.Label, "winrate:")[1], "%f", &existingWinrate); err != nil {
				log.Printf("解析现有winrate失败 for %s: %v", existing.Address, err)
				continue
			}

			// 如果新的 winrate 更高，更新记录
			if winrate > existingWinrate {
				uniqueAddresses[result.Address] = result
			}
		} else {
			// 如果地址不存在，添加新记录
			uniqueAddresses[result.Address] = result
		}
	}

	// 将map转换回切片
	var filteredResults []Result
	for _, result := range uniqueAddresses {
		filteredResults = append(filteredResults, result)
	}

	return filteredResults
}

func processAddressFiles() error {
	files, err := ioutil.ReadDir("ad_json")
	if err != nil {
		return fmt.Errorf("读取目录失败: %v", err)
	}

	// 读取已有的结果文件（如果存在）
	var allResults []Result
	if resultContent, err := ioutil.ReadFile("ad.json"); err == nil {
		json.Unmarshal(resultContent, &allResults)
	}

	// 遍历每个文件
	for _, file := range files {
		if !file.IsDir() && (filepath.Ext(file.Name()) == ".json" || filepath.Ext(file.Name()) == ".txt") {
			filePath := filepath.Join("ad_json", file.Name())
			
			addresses, err := readAddressesFromFile(filePath)
			if err != nil {
				log.Printf("处理文件 %s 失败: %v", file.Name(), err)
				continue
			}

			// 用于存储符合条件的地址
			var validAddresses []AddressItem
			var fileResults []Result

			// 处理每个地址
			for _, item := range addresses {
				if result, err := FetchAndAnalyzeData(item.Address); err != nil {
					log.Printf("处理地址 %s 失败: %v", item.Address, err)
				} else {
					if result != nil {
						fileResults = append(fileResults, *result)
						validAddresses = append(validAddresses, item)
					}
				}
				time.Sleep(2 * time.Second)
			}

			// 更新当前文件
			if len(validAddresses) > 0 {
				// 根据文件格式保存结果
				if filepath.Ext(file.Name()) == ".json" {
					updatedContent, err := json.MarshalIndent(validAddresses, "", "  ")
					if err != nil {
						log.Printf("更新文件 %s 失败: %v", file.Name(), err)
						continue
					}
					if err := ioutil.WriteFile(filePath, updatedContent, 0644); err != nil {
						log.Printf("保存文件 %s 失败: %v", file.Name(), err)
						continue
					}
				} else {
					// 保存TXT格式
					var txtContent strings.Builder
					txtContent.WriteString("address  label\n")
					for _, addr := range validAddresses {
						txtContent.WriteString(fmt.Sprintf("%s  %s\n", addr.Address, addr.Label))
					}
					if err := ioutil.WriteFile(filePath, []byte(txtContent.String()), 0644); err != nil {
						log.Printf("保存文件 %s 失败: %v", file.Name(), err)
						continue
					}
				}
				log.Printf("文件 %s 更新成功，保留了 %d 个符合条件的地址", file.Name(), len(validAddresses))
			} else {
				// 如果没有符合条件的地址，删除文件
				if err := os.Remove(filePath); err != nil {
					log.Printf("删除文件 %s 失败: %v", file.Name(), err)
				} else {
					log.Printf("文件 %s 中没有符合条件的地址，已删除", file.Name())
				}
			}

			// 将当前文件的结果添加到总结果中并立即保存
			if len(fileResults) > 0 {
				allResults = append(allResults, fileResults...)
				resultJSON, err := json.MarshalIndent(allResults, "", "  ")
				if err != nil {
					log.Printf("保存总结果失败: %v", err)
					continue
				}

				if err := ioutil.WriteFile("ad.json", resultJSON, 0644); err != nil {
					log.Printf("保存总结果文件失败: %v", err)
					continue
				}
				log.Printf("已更新总结果文件，当前共有 %d 个符合条件的地址", len(allResults))
			}
		}
	}

	// 在所有文件处理完成后，执行去重和过滤
	filteredResults := deduplicateAndFilter(allResults)

	// 保存JSON格式结果
	resultJSON, err := json.MarshalIndent(filteredResults, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON编码失败: %v", err)
	}

	if err := ioutil.WriteFile("ad.json", resultJSON, 0644); err != nil {
		return fmt.Errorf("保存JSON结果失败: %v", err)
	}

	// 创建 ad_txt 目录（如果不存在）
	if err := os.MkdirAll("ad_txt", 0755); err != nil {
		return fmt.Errorf("创建ad_txt目录失败: %v", err)
	}

	// 创建地址到原始标签的映射
	originalLabels := make(map[string]string)
	
	// 重新读取所有文件以获取原始标签
	files, err = ioutil.ReadDir("ad_json")
	if err != nil {
		return fmt.Errorf("读取目录失败: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && (filepath.Ext(file.Name()) == ".json" || filepath.Ext(file.Name()) == ".txt") {
			addresses, err := readAddressesFromFile(filepath.Join("ad_json", file.Name()))
			if err != nil {
				log.Printf("读取文件 %s 失败: %v", file.Name(), err)
				continue
			}
			
			for _, addr := range addresses {
				originalLabels[addr.Address] = addr.Label
			}
		}
	}

	// 保存TXT格式结果，使用原始标签
	var txtContent strings.Builder
	txtContent.WriteString("address  label\n")
	for _, result := range filteredResults {
		if originalLabel, exists := originalLabels[result.Address]; exists {
			txtContent.WriteString(fmt.Sprintf("%s  %s\n", result.Address, originalLabel))
		} else {
			log.Printf("警告: 找不到地址 %s 的原始标签", result.Address)
			// 如果找不到原始标签，使用当前的标签
			txtContent.WriteString(fmt.Sprintf("%s  %s\n", result.Address, result.Label))
		}
	}

	txtPath := filepath.Join("ad_txt", "addresses.txt")
	if err := ioutil.WriteFile(txtPath, []byte(txtContent.String()), 0644); err != nil {
		return fmt.Errorf("保存TXT结果失败: %v", err)
	}

	fmt.Printf("处理完成:\n")
	fmt.Printf("- 原始地址总数: %d\n", len(allResults))
	fmt.Printf("- 去重和过滤后地址数: %d\n", len(filteredResults))
	fmt.Printf("- JSON结果已保存到 ad.json\n")
	fmt.Printf("- TXT结果已保存到 ad_txt/addresses.txt\n")

	return nil
}

func main() {
	if err := processAddressFiles(); err != nil {
		log.Printf("错误: %v\n", err)
	}
}
