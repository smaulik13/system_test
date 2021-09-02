package tests

import (
	"fmt"
	"github.com/0chain/system_test/internal/config"
	"github.com/0chain/system_test/internal/model"
	"github.com/0chain/system_test/internal/utils"
	"github.com/stretchr/testify/assert"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestWalletRegisterAndBalanceOperations(t *testing.T) {
	walletConfigFilename := "wallet_TestWalletRegisterAndBalanceOperations_" + utils.RandomAlphaNumericString(10) + ".json"
	cliConfigFilename := "config_TestWalletRegisterAndBalanceOperations_" + utils.RandomAlphaNumericString(10) + ".yaml"

	systemTestConfig := GetConfig(t)
	cliConfig := model.Config{
		BlockWorker:             *systemTestConfig.DNSHostName + "/dns",
		SignatureScheme:         "bls0chain",
		MinSubmit:               50,
		MinConfirmation:         50,
		ConfirmationChainLength: 3,
		MaxTxnQuery:             5,
		QuerySleepTime:          5,
	}
	err := config.WriteConfig(cliConfigFilename, cliConfig)
	if err != nil {
		t.Errorf("Error when writing CLI config: %v", err)
	}

	t.Run("CLI output matches expected", func(t *testing.T) {
		output, err := utils.RegisterWallet(walletConfigFilename, cliConfigFilename)
		if err != nil {
			t.Errorf("An error occured registering a wallet due to error: %v", err)
		}

		assert.Equal(t, 4, len(output))
		assert.Equal(t, "ZCN wallet created", output[0])
		assert.Equal(t, "Creating related read pool for storage smart-contract...", output[1])
		assert.Equal(t, "Read pool created successfully", output[2])
		assert.Equal(t, "Wallet registered", output[3])
	})

	t.Run("Get wallet outputs expected", func(t *testing.T) {
		wallet, err := utils.GetWallet(t, walletConfigFilename, cliConfigFilename)

		if err != nil {
			t.Errorf("Error occured when retreiving wallet due to error: %v", err)
		}

		assert.NotNil(t, wallet.Client_id)
		assert.NotNil(t, wallet.Client_public_key)
		assert.NotNil(t, wallet.Encryption_public_key)
	})

	t.Run("Balance call fails due to zero ZCN in wallet", func(t *testing.T) {
		output, err := utils.GetBalance(walletConfigFilename, cliConfigFilename)
		if err == nil {
			t.Errorf("Expected initial getBalance operation to fail but was successful with output %v", strings.Join(output, "\n"))
		}

		assert.Equal(t, 1, len(output))
		assert.Equal(t, "Get balance failed.", output[0])
	})

	t.Run("Balance of 1 is returned after faucet execution", func(t *testing.T) {
		t.Run("Execute Faucet", func(t *testing.T) {
			output, err := utils.ExecuteFaucet(walletConfigFilename, cliConfigFilename)
			if err != nil {
				t.Errorf("Faucet execution failed due to error: %v", err)
			}

			assert.Equal(t, 1, len(output))
			matcher := regexp.MustCompile("Execute faucet smart contract success with txn : {2}([a-f0-9]{64})$")
			assert.Regexp(t, matcher, output[0], "Faucet execution output did not match expected")
			txnId := matcher.FindAllStringSubmatch(output[0], 1)[0][1]

			t.Run("Faucet Execution Verified", func(t *testing.T) {
				output, err = utils.VerifyTransaction(walletConfigFilename, cliConfigFilename, txnId)
				if err != nil {
					t.Errorf("Faucet verification failed due to error: %v", err)
				}

				assert.Equal(t, 1, len(output))
				assert.Equal(t, "Transaction verification success", output[0])
				t.Log("Faucet executed successful with txn id [" + txnId + "]")
			})
		})

		output, err := utils.GetBalance(walletConfigFilename, cliConfigFilename)
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, 1, len(output))
		assert.Regexp(t, regexp.MustCompile("Balance: 1 \\([0-9.]+ USD\\)$"), output[0])
	})
}

func TestMultiSigWalletRegisterAndBalanceOperations(t *testing.T) {
	msWalletConfigFilename := "wallet_TestMultiSigWalletRegisterAndBalanceOperations_" + "1" + ".json"
	cliConfigFilename := "config_TestMultiSigWalletRegisterAndBalanceOperations_" + "1" + ".yaml"

	systemTestConfig := GetConfig(t)

	cliConfig := model.Config{
		BlockWorker:             *systemTestConfig.DNSHostName + "/dns",
		SignatureScheme:         "bls0chain",
		MinSubmit:               50,
		MinConfirmation:         50,
		ConfirmationChainLength: 3,
		MaxTxnQuery:             5,
		QuerySleepTime:          5,
	}
	err := config.WriteConfig(cliConfigFilename, cliConfig)
	if err != nil {
		t.Errorf("Error when writing CLI config: %v", err)
	}

	t.Run("CLI output matches expected", func(t *testing.T) {

		testCases := []struct {
			NumSigners int
			Threshold  int
			Fail       bool
		}{
			{NumSigners: 3, Threshold: 0, Fail: true},
			{NumSigners: 3, Threshold: 2, Fail: false},
			{NumSigners: 3, Threshold: 3, Fail: false},
			{NumSigners: 3, Threshold: 4, Fail: true},
		}
		var newWalletCreated bool

		for i, tc := range testCases {

			output, err := utils.RunCommand(fmt.Sprintf(
				"./zwallet createmswallet --numsigners %d --threshold %d --silent --wallet %s --configDir ./temp --config %s",
				tc.NumSigners, tc.Threshold,
				msWalletConfigFilename,
				cliConfigFilename))
			if err != nil && !tc.Fail {
				fmt.Printf("An error occured registering a wallet due to error: %v\n", err)
			}

			// This is true for the first round only since the wallet is created here
			if i == 0 {
				if output[1] == "ZCN wallet created!!" {
					// This means a new wallet has been created
					newWalletCreated = true
					assert.Equal(t, "Creating related read pool for storage smart-contract...", output[2])
					assert.Equal(t, "Read pool created successfully", output[3])
				}
			}

			if tc.Fail {
				assert.NotEqual(t, "Creating and testing a multisig wallet is successful!", output[len(output)-1])

				// Check the error when threshold is greater than signers
				if tc.Threshold > tc.NumSigners {
					errMsg := fmt.Sprintf(
						"Error: given threshold (%d) is too high. Threshold has to be less than or equal to numsigners (%d)",
						tc.Threshold, tc.NumSigners,
					)
					assert.Equal(t, errMsg, output[len(output)-1])
				}
			}
			if !tc.Fail {
				base := 0
				if i == 0 {
					base += 5
				}
				// Total registered wallets = numsigners + 1 (additional wallet for multi-sig)
				msg := fmt.Sprintf("registering %d wallets ", tc.NumSigners+1)
				assert.Equal(t, msg, output[base])
				assert.Equal(t, "Creating and testing a multisig wallet is successful!", output[len(output)-1])
			}
		}

		// This test should only run if new wallet is created
		if newWalletCreated {
			t.Run("Balance call fails due to zero ZCN in wallet", func(t *testing.T) {
				output, err := utils.GetBalance(msWalletConfigFilename, cliConfigFilename)
				if err == nil {
					t.Errorf("Expected initial getBalance operation to fail but was successful with output %v", strings.Join(output, "\n"))
				}

				assert.Equal(t, 1, len(output))
				assert.Equal(t, "Get balance failed.", output[0])
			})
		}
	})

	// Since at least 2 test-cases create the multi-sig wallet, we can check it's contents
	t.Run("Get wallet outputs expected", func(t *testing.T) {
		wallet, err := utils.GetWallet(t, msWalletConfigFilename, cliConfigFilename)

		if err != nil {
			t.Errorf("Error occured when retreiving wallet due to error: %v", err)
		}

		assert.NotNil(t, wallet.Client_id)
		assert.NotNil(t, wallet.Client_public_key)
		assert.NotNil(t, wallet.Encryption_public_key)
	})

	t.Run("Balance increases by 1 after faucet execution", func(t *testing.T) {

		prevBalance := 0
		prevOutput, err := utils.GetBalance(msWalletConfigFilename, cliConfigFilename)
		// If the command fails, it means the balance is 0

		reBalance := regexp.MustCompile("Balance: ([0-9]+) \\([0-9.]+ USD\\)$")
		// If it passes, extract the balance
		if err == nil {
			matches := reBalance.FindStringSubmatch(prevOutput[0])
			if len(matches) > 1 {
				num, err := strconv.Atoi(matches[1])
				if err != nil {
					t.Errorf("Error on extracting balance: %v", err)
				} else {
					prevBalance = num
				}
			}
		}

		t.Run("Execute Faucet", func(t *testing.T) {
			output, err := utils.ExecuteFaucet(msWalletConfigFilename, cliConfigFilename)
			if err != nil {
				t.Errorf("Faucet execution failed due to error: %v", err)
			}

			assert.Equal(t, 1, len(output))
			matcher := regexp.MustCompile("Execute faucet smart contract success with txn : {2}([a-f0-9]{64})$")
			assert.Regexp(t, matcher, output[0], "Faucet execution output did not match expected")
			txnId := matcher.FindAllStringSubmatch(output[0], 1)[0][1]

			t.Run("Faucet Execution Verified", func(t *testing.T) {
				output, err = utils.VerifyTransaction(msWalletConfigFilename, cliConfigFilename, txnId)
				if err != nil {
					t.Errorf("Faucet verification failed due to error: %v", err)
				}

				assert.Equal(t, 1, len(output))
				assert.Equal(t, "Transaction verification success", output[0])
				t.Log("Faucet executed successful with txn id [" + txnId + "]")
			})
		})

		output, err := utils.GetBalance(msWalletConfigFilename, cliConfigFilename)
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, 1, len(output))

		balanceReStr := fmt.Sprintf("Balance: %d \\([0-9.]+ USD\\)$", prevBalance+1)
		assert.Regexp(t, regexp.MustCompile(balanceReStr), output[0])
	})
}
