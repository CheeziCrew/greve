package se.sundsvall.reactor.worker.cm;

import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.PostMapping;

@FeignClient(name = "care-management", url = "${integration.care-management.url}")
interface CareManagementClient {
	@PostMapping(path = "/{municipalityId}/errands/calculation/prepare")
	void prepare();
}
