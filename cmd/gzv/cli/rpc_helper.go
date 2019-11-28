//   Copyright (C) 2018 ZVChain
//
//   This program is free software: you can redistribute it and/or modify
//   it under the terms of the GNU General Public License as published by
//   the Free Software Foundation, either version 3 of the License, or
//   (at your option) any later version.
//
//   This program is distributed in the hope that it will be useful,
//   but WITHOUT ANY WARRANTY; without even the implied warranty of
//   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//   GNU General Public License for more details.
//
//   You should have received a copy of the GNU General Public License
//   along with this program.  If not, see <https://www.gnu.org/licenses/>.

package cli

import (
	"encoding/json"
	"fmt"
	browserlog "github.com/zvchain/zvchain/browser/log"
	"github.com/zvchain/zvchain/consensus/logical"
	"github.com/zvchain/zvchain/log"
	"github.com/zvchain/zvchain/tvm"
	"regexp"
	"strings"

	"github.com/zvchain/zvchain/common"
	"github.com/zvchain/zvchain/consensus/groupsig"
	"github.com/zvchain/zvchain/consensus/mediator"
	"github.com/zvchain/zvchain/core"
	"github.com/zvchain/zvchain/middleware/types"
)

var RegexpStr = `{"code":"TransferEvent = Event(\"transfer\")\nclass Token(object):\n    def __init__(self):\n^        self.name = \n^        self.symbol = \n^        self.decimal = \n^        self.totalSupply = \n        self.balanceOf = zdict()\n        self.allowance = zdict()\n        self.balanceOf[msg.sender] = self.totalSupply\n\n    def _transfer(self, _from, _to, _value):\n        if _to not in self.balanceOf:\n            self.balanceOf[_to] = 0\n        if _from not in self.balanceOf:\n            self.balanceOf[_from] = 0\n        # Whether the account balance meets the transfer amount\n        if self.balanceOf[_from] < _value:\n            return False\n        # Check if the transfer amount is legal\n        if _value <= 0:\n            return False\n        # Transfer\n        self.balanceOf[_from] -= _value\n        self.balanceOf[_to] += _value\n        return True\n\n    @register.public(str, int)\n    def transfer(self, _to, _value):\n        if self._transfer(msg.sender, _to, _value):\n            TransferEvent.emit(msg.sender, _to, _value)\n        else:\n            raise Exception(\"\")\n\n    @register.public(str, int)\n    def approve(self, _spender, _value):\n        if _value <= 0:\n            raise Exception('')\n        if msg.sender not in self.allowance:\n            self.allowance[msg.sender] = zdict()\n        self.allowance[msg.sender][_spender] = _value\n\n    @register.public(str, str, int)\n    def transfer_from(self, _from, _to, _value):\n        if _value > self.allowance[_from][msg.sender]:\n            raise Exception('')\n        self.allowance[_from][msg.sender] -= _value\n        if self._transfer(_from, _to, _value):\n            TransferEvent.emit(_from, _to, _value)\n        else:\n            raise Exception(\"\")\n\n    @register.public(int)\n    def burn(self, _value):\n        if _value <= 0:\n            raise Exception('')\n        if self.balanceOf[msg.sender] < _value:\n            raise Exception('')\n        self.balanceOf[msg.sender] -= _value\n        self.totalSupply -= _value","contract_name":"Token"}`

var TokenCodeReg = regexp.MustCompile("")

func ValidateTokenCode(str string) bool {
	return TokenCodeReg.MatchString(str)
}

func convertTransaction(tx *types.Transaction) *Transaction {
	var (
		gasLimit = uint64(0)
		gasPrice = uint64(0)
		value    = uint64(0)
	)
	if tx.GasLimit != nil {
		gasLimit = tx.GasLimit.Uint64()
	}
	if tx.GasPrice != nil {
		gasPrice = tx.GasPrice.Uint64()
	}
	if tx.Value != nil {
		value = tx.Value.Uint64()
	}
	trans := &Transaction{
		Hash:      tx.Hash,
		Source:    tx.Source,
		Target:    tx.Target,
		Type:      tx.Type,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		Data:      tx.Data,
		ExtraData: string(tx.ExtraData),
		Nonce:     tx.Nonce,
		Value:     common.RA2TAS(value),
	}
	return trans
}

func convertExecutedTransaction(executed *types.ExecutedTransaction) *ExecutedTransaction {
	rec := &Receipt{
		Status:            int(executed.Receipt.Status),
		CumulativeGasUsed: executed.Receipt.CumulativeGasUsed,
		Logs:              executed.Receipt.Logs,
		TxHash:            executed.Receipt.TxHash,
		ContractAddress:   executed.Receipt.ContractAddress,
		Height:            executed.Receipt.Height,
		TxIndex:           executed.Receipt.TxIndex,
	}
	return &ExecutedTransaction{
		Receipt:     rec,
		Transaction: convertTransaction(executed.Transaction),
	}

}

func convertBlockHeader(b *types.Block) *Block {
	bh := b.Header
	block := &Block{
		Height:  bh.Height,
		Hash:    bh.Hash,
		PreHash: bh.PreHash,
		CurTime: bh.CurTime.Local(),
		PreTime: bh.PreTime().Local(),
		Castor:  groupsig.DeserializeID(bh.Castor),
		Group:   bh.Group,
		Prove:   common.ToHex(bh.ProveValue),
		TotalQN: bh.TotalQN,
		TxNum:   uint64(len(b.Transactions)),
		//Qn: mediator.Proc.CalcBlockHeaderQN(bh),
		StateRoot:   bh.StateTree,
		TxRoot:      bh.TxTree,
		ReceiptRoot: bh.ReceiptTree,
		Random:      common.ToHex(bh.Random),
	}
	return block
}

func convertRewardTransaction(tx *types.Transaction) *RewardTransaction {
	if tx.Type != types.TransactionTypeReward {
		return nil
	}
	gSeed, ids, bhash, packFee, err := mediator.Proc.MainChain.GetRewardManager().ParseRewardTransaction(tx)
	if err != nil {
		return nil
	}
	targets := make([]groupsig.ID, len(ids))
	for i, id := range ids {
		targets[i] = groupsig.DeserializeID(id)
	}
	return &RewardTransaction{
		Hash:      tx.Hash,
		BlockHash: bhash,
		GroupSeed: gSeed,
		TargetIDs: targets,
		Value:     tx.Value.Uint64(),
		PackFee:   packFee.Uint64(),
	}
}

func genMinerBalance(id groupsig.ID, bh *types.BlockHeader) *MinerRewardBalance {
	mb := &MinerRewardBalance{
		ID: id,
	}
	db, err := mediator.Proc.MainChain.GetAccountDBByHash(bh.Hash)
	if err != nil {
		log.DefaultLogger.Errorf("GetAccountDBByHash err %v, hash %v", err, bh.Hash)
		return mb
	}
	mb.CurrBalance = db.GetBalance(id.ToAddress())
	preDB, err := mediator.Proc.MainChain.GetAccountDBByHash(bh.PreHash)
	if err != nil {
		log.DefaultLogger.Errorf("GetAccountDBByHash err %v hash %v", err, bh.PreHash)
		return mb
	}
	mb.PreBalance = preDB.GetBalance(id.ToAddress())
	return mb
}

func sendTransaction(trans *types.Transaction) error {
	if trans.Sign == nil {
		return fmt.Errorf("transaction sign is empty")
	}
	if ok, err := core.BlockChainImpl.GetTransactionPool().AddTransaction(trans); err != nil || !ok {
		log.DefaultLogger.Errorf("AddTransaction not ok or error:%s", err.Error())
		return err
	}
	return nil
}

func convertGroup(g types.GroupI) *Group {

	mems := make([]string, 0)
	for _, mem := range g.Members() {
		memberStr := groupsig.DeserializeID(mem.ID()).GetAddrString()
		mems = append(mems, memberStr)
	}
	gh := g.Header()

	return &Group{
		Seed:          gh.Seed(),
		BeginHeight:   gh.WorkHeight(),
		DismissHeight: gh.DismissHeight(),
		Threshold:     int32(gh.Threshold()),
		Members:       mems,
		MemSize:       len(mems),
		GroupHeight:   gh.GroupHeight(),
	}

}

func parseABI(code string) []tvm.ABIVerify {

	ABIs := make([]tvm.ABIVerify, 0)

	stringSlice := strings.Split(code, "\n")
	for k, targetString := range stringSlice {
		targetString = strings.TrimSpace(targetString)
		if strings.HasPrefix(targetString, "@register.public") {
			params := strings.TrimPrefix(targetString, "@register.public")
			params = params[1 : len(params)-1]
			args := strings.Split(params, ",")
			argsNew := make([]string, 0)
			for _, arg := range args {
				arg = strings.TrimSpace(arg)
				if len(arg) > 0 {
					argsNew = append(argsNew, arg)
				}
			}

			funcName := ""
			if k+1 < len(stringSlice) {
				funcLine := stringSlice[k+1]
				funcLine = strings.TrimSpace(funcLine)
				if strings.HasPrefix(funcLine, "def") {
					funcLine = strings.TrimPrefix(funcLine, "def")
					funcLine = strings.TrimSpace(funcLine)

					for m, v := range funcLine {
						if v == '(' {
							funcName = funcLine[:m]
							funcName = strings.TrimSpace(funcName)
						}
					}
					abi := tvm.ABIVerify{
						FuncName: funcName,
						Args:     argsNew,
					}
					ABIs = append(ABIs, abi)
				}
			}
		}
	}
	return ABIs
}

func getMorts(p logical.Processor) (string, []MortGage) {
	morts := make([]MortGage, 0)
	t := "--"
	addr := common.BytesToAddress(p.GetMinerID().Serialize())
	proposalInfo := core.MinerManagerImpl.GetLatestMiner(addr, types.MinerTypeProposal)
	if proposalInfo != nil {
		morts = append(morts, *NewMortGageFromMiner(proposalInfo))
		if proposalInfo.IsActive() {
			t = "proposal role"
		}
	}
	verifyInfo := core.MinerManagerImpl.GetLatestMiner(addr, types.MinerTypeVerify)
	if verifyInfo != nil {
		morts = append(morts, *NewMortGageFromMiner(verifyInfo))
		if verifyInfo.IsActive() {
			t += " verify role"
		}
	}
	return t, morts
}

func IsTokenContract(contractAddr common.Address) bool {
	chain := core.BlockChainImpl
	db, err := chain.LatestAccountDB()
	if err != nil {
		browserlog.BrowserLog.Error("isTokenContract: ", err)
		return false
	}
	code := db.GetCode(contractAddr)
	contract := tvm.Contract{}
	err = json.Unmarshal(code, &contract)
	if err != nil {
		browserlog.BrowserLog.Error("isTokenContract: ", err)
		return false
	}
	if HasTransferFunc(contract.Code) {
		symbol := db.GetData(contractAddr, []byte("symbol"))
		if len(symbol) >= 1 && symbol[0] == 's' {
			return true
		}
	}
	return false
}

func HasTransferFunc(code string) bool {
	stringSlice := strings.Split(code, "\n")
	for k, targetString := range stringSlice {
		targetString = strings.TrimSpace(targetString)
		if strings.HasPrefix(targetString, "@register.public") {
			if len(stringSlice) > k+1 {
				if strings.Index(stringSlice[k+1], " transfer(") != -1 {
					return true
				}
			}
		}
	}
	return false
}

func IsTokenContractFromZiWeiBao(contractAddr common.Address) bool {
	chain := core.BlockChainImpl
	db, err := chain.LatestAccountDB()
	if err != nil {
		browserlog.BrowserLog.Error("IsTokenContractFromZiWeiBao: ", err)
		return false
	}
	code := db.GetCode(contractAddr)
	RegexpStrSlice := strings.Split(RegexpStr, "\\n")
	codeSlice := strings.Split(string(code), "\\n")

	if len(RegexpStrSlice) != len(codeSlice) {
		browserlog.BrowserLog.Error("len(RegexpStrSlice) != len(codeSlice)")
		return false
	}

	for k, v1 := range RegexpStrSlice {
		if k < 3 || k > 6 {
			if k < len(codeSlice) && v1 != codeSlice[k] {
				browserlog.BrowserLog.Error("v1 != codeSlice[k]. v1=:", v1, " ,codeSlice[k]=", codeSlice[k])
				return false
			}
		} else if k >= 3 && k <= 6 {
			TokenCodeReg = regexp.MustCompile(v1)
			if !ValidateTokenCode(codeSlice[k]) {
				browserlog.BrowserLog.Error("v:", codeSlice[k], " not match RegexpStrSlice:", v1)
				return false
			}
		}
	}
	return true
}
