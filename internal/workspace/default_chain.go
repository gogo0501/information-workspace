// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

type DefaultChain struct{ service *Service }

func NewDefaultChain(service *Service) *DefaultChain { return &DefaultChain{service: service} }

func (c *DefaultChain) Run(userID int64, input string) (*Answer, error) {
	answer, err := c.service.Ask(userID, input)
	if answer != nil {
		answer.Route = "default"
	}
	return answer, err
}
