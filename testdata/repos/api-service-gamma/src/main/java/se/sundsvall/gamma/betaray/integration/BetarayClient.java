package se.sundsvall.gamma.betaray.integration;

import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;

@FeignClient(
	name = "betaray",
	url = "${integration.betaray.base-url}")
interface BetarayClient {

	@GetMapping("/{municipalityId}/rays")
	String getRays(String municipalityId);
}
