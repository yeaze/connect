// Copyright 2024 Redpanda Data, Inc.
//
// Licensed as a Redpanda Enterprise file under the Redpanda Community
// License (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
// https://github.com/redpanda-data/connect/blob/main/licenses/rcl.md

package enterprise

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

var (
	errTopicAlreadyExists = errors.New("topic already exists")
)

func createTopic(ctx context.Context, topic string, inputClient *kgo.Client, outputClient *kgo.Client) error {
	outputAdminClient := kadm.NewClient(outputClient)

	if topics, err := outputAdminClient.ListTopics(ctx, topic); err != nil {
		return fmt.Errorf("failed to fetch topic %q from output broker: %s", topic, err)
	} else {
		if topics.Has(topic) {
			return errTopicAlreadyExists
		}
	}

	inputAdminClient := kadm.NewClient(inputClient)
	var inputTopic kadm.TopicDetail
	if topics, err := inputAdminClient.ListTopics(ctx, topic); err != nil {
		return fmt.Errorf("failed to fetch topic %q from source broker: %s", topic, err)
	} else {
		inputTopic = topics[topic]
	}

	partitions := int32(len(inputTopic.Partitions))
	if partitions == 0 {
		partitions = -1
	}
	replicationFactor := int16(inputTopic.Partitions.NumReplicas())
	if replicationFactor == 0 {
		replicationFactor = -1
	}

	if _, err := outputAdminClient.CreateTopic(ctx, partitions, replicationFactor, nil, topic); err != nil {
		if !errors.Is(err, kerr.TopicAlreadyExists) {
			return fmt.Errorf("failed to create topic %q: %s", topic, err)
		}
	}

	return nil
}

func createACLs(ctx context.Context, topic string, inputClient *kgo.Client, outputClient *kgo.Client) error {
	inputAdminClient := kadm.NewClient(inputClient)
	outputAdminClient := kadm.NewClient(outputClient)

	// Only topic ACLs are migrated, group ACLs are not migrated.
	// Users are not migrated because we can't read passwords.

	aclBuilder := kadm.NewACLs().Topics(topic).
		ResourcePatternType(kadm.ACLPatternLiteral).Operations().Allow().Deny().AllowHosts().DenyHosts()
	var inputACLResults kadm.DescribeACLsResults
	var err error
	if inputACLResults, err = inputAdminClient.DescribeACLs(ctx, aclBuilder); err != nil {
		return fmt.Errorf("failed to fetch ACLs for topic %q: %s", topic, err)
	}

	if len(inputACLResults) > 1 {
		return fmt.Errorf("received unexpected number of ACL results for topic %q: %d", topic, len(inputACLResults))
	}

	for _, acl := range inputACLResults[0].Described {
		builder := kadm.NewACLs()

		if acl.Permission == kmsg.ACLPermissionTypeAllow && acl.Operation == kmsg.ACLOperationWrite {
			// ALLOW WRITE ACLs for topics are not migrated.
			continue
		}

		op := acl.Operation
		if op == kmsg.ACLOperationAll {
			// ALLOW ALL ACLs for topics are downgraded to ALLOW READ.
			op = kmsg.ACLOperationRead
		}
		switch acl.Permission {
		case kmsg.ACLPermissionTypeAllow:
			builder = builder.Allow(acl.Principal).AllowHosts(acl.Host).Topics(acl.Name).ResourcePatternType(acl.Pattern).Operations(op)
		case kmsg.ACLPermissionTypeDeny:
			builder = builder.Deny(acl.Principal).DenyHosts(acl.Host).Topics(acl.Name).ResourcePatternType(acl.Pattern).Operations(op)
		}

		// Attempting to overwrite existing ACLs is idempotent and doesn't seem to raise an error.
		if _, err := outputAdminClient.CreateACLs(ctx, builder); err != nil {
			return fmt.Errorf("failed to create ACLs for topic %q: %s", topic, err)
		}
	}

	return nil
}