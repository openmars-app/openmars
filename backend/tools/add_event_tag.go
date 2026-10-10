package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Card struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Cost        int             `json:"cost"`
	Description json.RawMessage `json:"description"`
	GameModes   []string        `json:"gameModes"`
	Tags        []string        `json:"tags,omitempty"`
	// All other fields will be preserved with RawMessage
	Other map[string]json.RawMessage `json:"-"`
}

// UnmarshalJSON custom unmarshaler to preserve all fields
func (c *Card) UnmarshalJSON(data []byte) error {
	// First unmarshal into a map to get all fields
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	// Extract the known fields
	type Alias Card
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Store remaining fields
	c.Other = make(map[string]json.RawMessage)
	knownFields := map[string]bool{
		"id": true, "name": true, "type": true, "cost": true,
		"description": true, "pack": true, "tags": true,
	}
	for key, value := range rawMap {
		if !knownFields[key] {
			c.Other[key] = value
		}
	}

	return nil
}

// MarshalJSON custom marshaler to include all fields
func (c Card) MarshalJSON() ([]byte, error) {
	// Start with a map of all fields
	result := make(map[string]interface{})

	// Add known fields
	result["id"] = c.ID
	result["name"] = c.Name
	result["type"] = c.Type
	result["cost"] = c.Cost
	result["description"] = c.Description
	result["gameModes"] = c.GameModes
	if len(c.Tags) > 0 {
		result["tags"] = c.Tags
	}

	// Add other fields
	for key, value := range c.Other {
		var v interface{}
		if err := json.Unmarshal(value, &v); err != nil {
			return nil, fmt.Errorf("failed to unmarshal field %q: %w", key, err)
		}
		result[key] = v
	}

	return json.Marshal(result)
}

func main() {
	// Read the cards JSON file
	filePath := "../assets/cards.json"
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Unmarshal into slice of cards
	var cards []Card
	if err := json.Unmarshal(data, &cards); err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
		os.Exit(1)
	}

	// Add "event" tag to all event cards
	updatedCount := 0
	for i := range cards {
		if cards[i].Type == "event" {
			// Check if "event" tag already exists
			hasEventTag := false
			for _, tag := range cards[i].Tags {
				if tag == "event" {
					hasEventTag = true
					break
				}
			}

			// Add "event" tag if it doesn't exist
			if !hasEventTag {
				cards[i].Tags = append(cards[i].Tags, "event")
				updatedCount++
				fmt.Printf("Added 'event' tag to: %s (%s)\n", cards[i].Name, cards[i].ID)
			}
		}
	}

	// Marshal back to JSON with indentation
	output, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	// Write back to file
	if err := os.WriteFile(filePath, output, 0644); err != nil {
		fmt.Printf("Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSuccessfully updated %d event cards with 'event' tag\n", updatedCount)
}
