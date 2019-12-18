////   Copyright (C) 2018 ZVChain
////
////   This program is free software: you can redistribute it and/or modify
////   it under the terms of the GNU General Public License as published by
////   the Free Software Foundation, either version 3 of the License, or
////   (at your option) any later version.
////
////   This program is distributed in the hope that it will be useful,
////   but WITHOUT ANY WARRANTY; without even the implied warranty of
////   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
////   GNU General Public License for more details.
////
////   You should have received a copy of the GNU General Public License
////   along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
package core

import (
	"github.com/zvchain/zvchain/consensus/groupsig"
	"github.com/zvchain/zvchain/core/group"
	"testing"
)

func TestValidateHeaders(t *testing.T) {
	err := initContext4Test(t)
	defer clearSelf(t)
	if err != nil {
		t.Fatalf("failed to initContext4Test")
	}
	chain := BlockChainImpl
	buildChain(100, chain)

	GroupManagerImpl = group.NewManager(chain, NewConsensusHelper4Test(groupsig.ID{}))
	err = validateHeaders(chain, chain.getTopBlockByHeight(90).Header.Hash)
	if err != nil {
		t.Error(err)
	}
}

func TestValidateStateTree(t *testing.T) {
	err := initContext4Test(t)
	defer clearSelf(t)
	if err != nil {
		t.Fatalf("failed to initContext4Test")
	}
	chain := BlockChainImpl
	buildChain(100, chain)
	err = validateStateDb(chain, chain.getTopBlockByHeight(90).Header)
	if err != nil {
		t.Error(err)
	}

}