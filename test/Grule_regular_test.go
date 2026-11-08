// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

package test

import (
	"HomeWaf/innerbean"
	"HomeWaf/internal/thirdparty/grule/ast"
	"HomeWaf/internal/thirdparty/grule/builder"
	"HomeWaf/internal/thirdparty/grule/engine"
	"HomeWaf/internal/thirdparty/grule/pkg"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

func TestGruleRegular(t *testing.T) {
	myFact := &innerbean.WebLog{
		SRC_IP: "127a",
	}
	dataCtx := ast.NewDataContext()
	err := dataCtx.Add("MF", myFact)
	assert.NoError(t, err)

	// Prepare knowledgebase library and load it with our rule.
	knowledgeLibrary := ast.NewKnowledgeLibrary()
	ruleBuilder := builder.NewRuleBuilder(knowledgeLibrary)

	drls := `

rule R77a9d95d14624d4baaa9ef7af3061404 "试试344" salience 10 {
    when 
        MF.SRC_IP.MatchString("^[0-9]*$")
    then
        
		Retract("R77a9d95d14624d4baaa9ef7af3061404");
} 
`

	byteArr := pkg.NewBytesResource([]byte(drls))
	err = ruleBuilder.BuildRuleFromResource("Tutorial", "0.0.1", byteArr)
	assert.NoError(t, err)

	knowledgeBase, err := knowledgeLibrary.NewKnowledgeBaseInstance("Tutorial", "0.0.1")
	assert.NoError(t, err)

	my_engine := engine.NewGruleEngine()
	processType := "match"
	if processType == "match" {
		matchrules, err := my_engine.FetchMatchingRules(dataCtx, knowledgeBase)
		if err != nil {
			log.Fatal(err)
		} else {
			log.Println(len(matchrules))
			for _, v := range matchrules {
				log.Println(v.RuleName)

				log.Println(v.RuleDescription)
			}
		}
	} else {
		err := my_engine.Execute(dataCtx, knowledgeBase)
		if err != nil {
			log.Fatal(err)
		}
	}

}
