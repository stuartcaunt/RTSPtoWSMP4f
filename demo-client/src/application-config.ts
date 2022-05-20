export class ApplicationConfig {

  server: {
    port: number,
    host: string,
  }

  logging: {
    level: string;
    timezone: string;
  };

  constructor(data?: Partial<ApplicationConfig>) {
    Object.assign(this, data);
  }
}

let applicationConfig: ApplicationConfig;

export function APPLICATION_CONFIG(): ApplicationConfig {
  if (applicationConfig == null) {
    applicationConfig = {
      server: {
        port: process.env.VISA_SECURITY_GROUPS_SERVER_PORT == null ? 4000 : +process.env.VISA_SECURITY_GROUPS_SERVER_PORT,
        host: process.env.VISA_SECURITY_GROUPS_SERVER_HOST == null ? 'localhost' : process.env.VISA_SECURITY_GROUPS_SERVER_HOST,
      },
      logging: {
        level: process.env.VISA_SECURITY_GROUPS_LOG_LEVEL == null ? 'info' : process.env.VISA_SECURITY_GROUPS_LOG_LEVEL,
        timezone: process.env.VISA_SECURITY_GROUPS_LOG_TIMEZONE,
      }
    };
  }

  return applicationConfig;
}
