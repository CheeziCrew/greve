package se.sundsvall.alpha.integration.betaray;

import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;

@FeignClient(
	name = "beta-ray",
	url = "${integration.beta-ray.base-url}",
	configuration = BetaRayConfiguration.class)
interface BetaRayClient {

	@GetMapping("/{municipalityId}/rays")
	String getRays(String municipalityId);
}
